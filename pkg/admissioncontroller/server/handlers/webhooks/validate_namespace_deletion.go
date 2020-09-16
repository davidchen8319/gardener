// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/operation/common"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type namespaceDeletionHandler struct {
	k8sGardenClient kubernetes.Interface

	codecs serializer.CodecFactory
}

const waitForCachesToSyncTimeout = 5 * time.Minute

// NewValidateNamespaceDeletionHandler creates a new handler for validating namespace deletions.
func NewValidateNamespaceDeletionHandler(ctx context.Context, k8sGardenClient kubernetes.Interface) (http.HandlerFunc, error) {
	// Initialize caches here to ensure http requests can be served quicker with pre-syncronized caches.
	var hasSyncFuncs []cache.InformerSynced
	informer, err := k8sGardenClient.Cache().GetInformer(ctx, &corev1.Secret{})
	if err != nil {
		return nil, err
	}
	hasSyncFuncs = append(hasSyncFuncs, informer.HasSynced)
	informer, err = k8sGardenClient.Cache().GetInformer(ctx, &gardencorev1beta1.Shoot{})
	if err != nil {
		return nil, err
	}
	hasSyncFuncs = append(hasSyncFuncs, informer.HasSynced)

	timeoutCtx, cancel := context.WithTimeout(ctx, waitForCachesToSyncTimeout)
	defer cancel()
	if !cache.WaitForCacheSync(timeoutCtx.Done(), hasSyncFuncs...) {
		return nil, errors.New("timed out waiting for caches to sync")
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(corev1.AddToScheme(scheme))

	h := &namespaceDeletionHandler{k8sGardenClient, serializer.NewCodecFactory(scheme)}
	return h.ValidateNamespaceDeletion, nil
}

// ValidateNamespaceDeletion is a HTTP handler for validating whether a namespace deletion is allowed or not.
func (h *namespaceDeletionHandler) ValidateNamespaceDeletion(w http.ResponseWriter, r *http.Request) {
	var (
		body []byte

		deserializer   = h.codecs.UniversalDeserializer()
		receivedReview = admissionv1beta1.AdmissionReview{}

		wantedContentType = runtime.ContentTypeJSON
		wantedOperation   = admissionv1beta1.Delete
	)

	// Read HTTP request body into variable.
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	// Verify that the correct content-type header has been sent.
	if contentType := r.Header.Get("Content-Type"); contentType != wantedContentType {
		err := fmt.Errorf("contentType=%s, expect %s", contentType, wantedContentType)
		logger.Logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}

	// Deserialize HTTP request body into admissionv1beta1.AdmissionReview object.
	if _, _, err := deserializer.Decode(body, nil, &receivedReview); err != nil {
		logger.Logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}

	// If the request field is empty then do not admit (invalid body).
	if receivedReview.Request == nil {
		err := fmt.Errorf("invalid request body (missing admission request)")
		logger.Logger.Errorf(err.Error())
		respond(w, errToAdmissionResponse(err))
		return
	}
	// If the request does not indicate the correct operation (DELETE) we allow the review without further doing.
	if receivedReview.Request.Operation != wantedOperation {
		respond(w, admissionResponse(true, ""))
		return
	}

	// Now that all checks have been passed we can actually validate the admission request.
	reviewResponse := h.admitNamespaces(receivedReview.Request)
	if !reviewResponse.Allowed && reviewResponse.Result != nil {
		logger.Logger.Infof("Rejected 'DELETE namespace' request of user '%s': %v", receivedReview.Request.UserInfo.Username, reviewResponse.Result.Message)
	}
	respond(w, reviewResponse)
}

// admitNamespaces does only allow the request if no Shoots  exist in this
// specific namespace anymore.
func (h *namespaceDeletionHandler) admitNamespaces(request *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	namespaceResource := metav1.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
	if request.Resource != namespaceResource {
		return errToAdmissionResponse(fmt.Errorf("expect resource to be %s", namespaceResource))
	}

	// Determine project object for given namespace.
	project, err := common.ProjectForNamespaceWithClient(ctx, h.k8sGardenClient.Client(), request.Name)
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Namespace does not belong to a project. Deletion is allowed.
			return admissionResponse(true, "")
		}
		return errToAdmissionResponse(err)
	}

	// We do not receive the namespace object in the `.object` field of the admission request. Hence, we need to get it ourselves.
	namespace := &corev1.Namespace{}
	err = h.k8sGardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: request.Name}, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return admissionResponse(true, "")
		}
		return errToAdmissionResponse(err)
	}

	switch {
	case namespace.DeletionTimestamp != nil:
		// Namespace is already marked to be deleted so we can allow the request.
		return admissionResponse(true, "")
	case project.DeletionTimestamp != nil:
		namespaceEmpty, err := h.isNamespaceEmpty(ctx, namespace.Name)
		if err != nil {
			return errToAdmissionResponse(err)
		}

		if namespaceEmpty {
			return admissionResponse(true, "")
		}
		return admissionResponse(false, fmt.Sprintf("Deletion of namespace %q is not permitted (there are still Shoots).", namespace.Name))
	}

	// Namespace is not yet marked to be deleted and project is not marked as well. We do not admit and respond that namespace deletion is only
	// allowed via project deletion.
	return admissionResponse(false, fmt.Sprintf("Direct deletion of namespace %q is not permitted (you must delete the corresponding project %q).", namespace.Name, project.Name))
}

func (h *namespaceDeletionHandler) isNamespaceEmpty(ctx context.Context, namespace string) (bool, error) {
	shoots := &gardencorev1beta1.ShootList{}
	if err := h.k8sGardenClient.Client().List(ctx, shoots, client.InNamespace(namespace)); err != nil {
		return false, err
	}

	return len(shoots.Items) == 0, nil
}
