// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package shoot_creation

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestShootApplications(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Create Test Suite")
}
