// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0
//go:generate mockgen -package v1 -destination=mocks.go k8s.io/client-go/kubernetes/typed/core/v1 PodInterface,NodeInterface,NamespaceInterface

package v1
