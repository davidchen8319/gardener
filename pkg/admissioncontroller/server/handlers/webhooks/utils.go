// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package webhooks

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/gardener/gardener/pkg/logger"

	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func admissionResponse(allowed bool, msg string) *admissionv1beta1.AdmissionResponse {
	response := &admissionv1beta1.AdmissionResponse{
		Allowed: allowed,
	}

	if msg != "" {
		response.Result = &metav1.Status{
			Message: msg,
		}
	}

	return response
}

func errToAdmissionResponse(err error) *admissionv1beta1.AdmissionResponse {
	return admissionResponse(false, err.Error())
}

func respond(w http.ResponseWriter, response *admissionv1beta1.AdmissionResponse) {
	responseObj := admissionv1beta1.AdmissionReview{}
	if response != nil {
		responseObj.Response = response
	}

	jsonResponse, err := json.Marshal(responseObj)
	if err != nil {
		logger.Logger.Error(err)
	}
	if _, err := w.Write(jsonResponse); err != nil {
		logger.Logger.Error(err)
	}
}

// DecodeAdmissionRequest decodes the given http request into an admission request.
// An error is returned if the request exceeds the given limit.
func DecodeAdmissionRequest(r *http.Request, decoder runtime.Decoder, into *admissionv1beta1.AdmissionReview, limit int64) error {
	// Read HTTP request body into variable.
	var (
		body              []byte
		wantedContentType = runtime.ContentTypeJSON
		// Increase limit by 1 (spare capacity) to determine if the limit was exceeded or right on the mark after reading.
		lr = &io.LimitedReader{R: r.Body, N: limit + 1}
	)

	if r.Body != nil {
		data, err := ioutil.ReadAll(lr)
		if err != nil {
			return err
		}
		if lr.N <= 0 {
			return apierrors.NewRequestEntityTooLargeError(fmt.Sprintf("limit is %d", limit))
		}
		body = data
	}

	// Verify that the correct content-type header has been sent.
	if contentType := r.Header.Get("Content-Type"); contentType != wantedContentType {
		return fmt.Errorf("contentType=%s, expect %s", contentType, wantedContentType)
	}

	// Deserialize HTTP request body into admissionv1beta1.AdmissionReview object.
	if _, _, err := decoder.Decode(body, nil, into); err != nil {
		return err
	}

	// If the request field is empty then do not admit (invalid body).
	if into.Request == nil {
		return fmt.Errorf("invalid request body (missing admission request)")
	}

	return nil
}
