// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package secret_test

import (
	"context"
	"testing"

	secretutil "github.com/gardener/gardener/extensions/pkg/util/secret"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSecretUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Secret utils suite")
}

var _ = Describe("Secret", func() {

	Context("#IsSecretInUseByShoot", func() {
		const namespace = "namespace"

		var (
			scheme *runtime.Scheme

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: namespace,
				},
			}
			secretBinding = &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secretbinding",
					Namespace: namespace,
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespace,
				},
				Spec: gardencorev1beta1.ShootSpec{
					Provider: gardencorev1beta1.Provider{
						Type: "gcp",
					},
					SecretBindingName: secretBinding.Name,
				},
			}
		)

		BeforeEach(func() {
			scheme = runtime.NewScheme()
			Expect(corev1.AddToScheme(scheme)).NotTo(HaveOccurred())
			Expect(gardencorev1beta1.AddToScheme(scheme)).NotTo(HaveOccurred())
		})

		It("should return false when the Secret is not used", func() {
			client := fake.NewFakeClientWithScheme(
				scheme,
				secret,
				secretBinding,
			)

			isUsed, err := secretutil.IsSecretInUseByShoot(context.TODO(), client, secret, "gcp")
			Expect(isUsed).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return false when the Secret is in use but the provider does not match", func() {
			client := fake.NewFakeClientWithScheme(
				scheme,
				secret,
				secretBinding,
				shoot,
			)

			isUsed, err := secretutil.IsSecretInUseByShoot(context.TODO(), client, secret, "other")
			Expect(isUsed).To(BeFalse())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return true when the Secret is in use by Shoot with the given provider", func() {
			client := fake.NewFakeClientWithScheme(
				scheme,
				secret,
				secretBinding,
				shoot,
			)

			isUsed, err := secretutil.IsSecretInUseByShoot(context.TODO(), client, secret, "gcp")
			Expect(isUsed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return true when the Secret is in use by Shoot from another namespace", func() {
			secret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret",
					Namespace: "another-namespace",
				},
			}
			secretBinding := &gardencorev1beta1.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secretbinding",
					Namespace: namespace,
				},
				SecretRef: corev1.SecretReference{
					Name:      secret.Name,
					Namespace: secret.Namespace,
				},
			}

			client := fake.NewFakeClientWithScheme(
				scheme,
				secret,
				secretBinding,
				shoot,
			)

			isUsed, err := secretutil.IsSecretInUseByShoot(context.TODO(), client, secret, "gcp")
			Expect(isUsed).To(BeTrue())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
