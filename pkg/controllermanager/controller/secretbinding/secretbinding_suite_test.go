// SPDX-FileCopyrightText: 2018 2020 SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
// SPDX-License-Identifier: Apache-2.0

package secretbinding_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSecretBinding(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ControllerManager SecretBinding Controller Suite")
}
