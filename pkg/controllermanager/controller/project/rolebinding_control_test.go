// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package project

import (
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
)

var _ = Describe("#roleBindingDelete", func() {
	const ns = "test"

	var (
		c           *Controller
		indexer     cache.Indexer
		queue       workqueue.RateLimitingInterface
		proj        *gardencorev1beta1.Project
		rolebinding *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		// This should not be here!!! Hidden dependency!!!
		logger.Logger = logger.NewNopLogger()

		indexer = cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{})
		queue = workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())
		proj = &gardencorev1beta1.Project{
			ObjectMeta: metav1.ObjectMeta{Name: "project-1"},
			Spec: gardencorev1beta1.ProjectSpec{
				Namespace: pointer.StringPtr(ns),
			},
		}
		rolebinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "role-1", Namespace: ns},
		}
		c = &Controller{
			projectLister: gardencorelisters.NewProjectLister(indexer),
			projectQueue:  queue,
		}
	})

	AfterEach(func() {
		queue.ShutDown()
	})

	It("should not requeue random rolebinding", func() {
		Expect(indexer.Add(proj)).ToNot(HaveOccurred())

		c.roleBindingDelete(rolebinding)

		Expect(queue.Len()).To(Equal(0), "no items in the queue")
	})

	DescribeTable("requeue when rolebinding is",
		func(roleBindingName string) {
			rolebinding.Name = roleBindingName
			Expect(indexer.Add(proj)).ToNot(HaveOccurred())

			c.roleBindingDelete(rolebinding)

			Expect(queue.Len()).To(Equal(1), "only one item in queue")
			actual, _ := queue.Get()
			Expect(actual).To(Equal("project-1"))
		},

		Entry("project-member", "gardener.cloud:system:project-member"),
		Entry("project-viewer", "gardener.cloud:system:project-viewer"),
		Entry("custom role", "gardener.cloud:extension:project:project-1:foo"),
	)

	DescribeTable("no requeue when project is being deleted and rolebinding is",
		func(roleBindingName string) {
			now := metav1.Now()
			proj.DeletionTimestamp = &now
			rolebinding.Name = roleBindingName
			Expect(indexer.Add(proj)).ToNot(HaveOccurred())

			c.roleBindingDelete(rolebinding)

			Expect(queue.Len()).To(Equal(0), "no projects in queue")
		},

		Entry("project-member", "gardener.cloud:system:project-member"),
		Entry("project-viewer", "gardener.cloud:system:project-viewer"),
		Entry("custom role", "gardener.cloud:extension:project:project-1:foo"),
	)
})
