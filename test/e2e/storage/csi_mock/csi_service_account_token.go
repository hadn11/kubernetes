/*
Copyright 2022 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package csi_mock

import (
	"context"
	"time"

	"github.com/onsi/ginkgo/v2"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/kubernetes/test/e2e/storage/utils"
	admissionapi "k8s.io/pod-security-admission/api"
	utilptr "k8s.io/utils/pointer"
)

var _ = utils.SIGDescribe("CSI Mock volume service account token", func() {
	f := framework.NewDefaultFramework("csi-mock-volumes-service-token")
	f.NamespacePodSecurityEnforceLevel = admissionapi.LevelPrivileged
	m := newMockDriverSetup(f)

	ginkgo.Context("CSIServiceAccountToken", func() {
		var (
			err error
		)
		tests := []struct {
			desc                  string
			deployCSIDriverObject bool
			tokenRequests         []storagev1.TokenRequest
		}{
			{
				desc:                  "token should not be plumbed down when csiServiceAccountTokenEnabled=false",
				deployCSIDriverObject: true,
				tokenRequests:         nil,
			},
			{
				desc:                  "token should not be plumbed down when CSIDriver is not deployed",
				deployCSIDriverObject: false,
				tokenRequests:         []storagev1.TokenRequest{{}},
			},
			{
				desc:                  "token should be plumbed down when csiServiceAccountTokenEnabled=true",
				deployCSIDriverObject: true,
				tokenRequests:         []storagev1.TokenRequest{{ExpirationSeconds: utilptr.Int64Ptr(60 * 10)}},
			},
		}
		for _, test := range tests {
			test := test
			csiServiceAccountTokenEnabled := test.tokenRequests != nil
			ginkgo.It(test.desc, func(ctx context.Context) {
				m.init(testParameters{
					registerDriver:    test.deployCSIDriverObject,
					tokenRequests:     test.tokenRequests,
					requiresRepublish: &csiServiceAccountTokenEnabled,
				})

				ginkgo.DeferCleanup(m.cleanup)

				_, _, pod := m.createPod(pvcReference)
				if pod == nil {
					return
				}
				err = e2epod.WaitForPodNameRunningInNamespace(m.cs, pod.Name, pod.Namespace)
				framework.ExpectNoError(err, "Failed to start pod: %v", err)

				// sleep to make sure RequiresRepublish triggers more than 1 NodePublishVolume
				numNodePublishVolume := 1
				if test.deployCSIDriverObject && csiServiceAccountTokenEnabled {
					time.Sleep(5 * time.Second)
					numNodePublishVolume = 2
				}

				ginkgo.By("Deleting the previously created pod")
				err = e2epod.DeletePodWithWait(m.cs, pod)
				framework.ExpectNoError(err, "while deleting")

				ginkgo.By("Checking CSI driver logs")
				err = checkPodLogs(m.driver.GetCalls, pod, false, false, false, test.deployCSIDriverObject && csiServiceAccountTokenEnabled, numNodePublishVolume)
				framework.ExpectNoError(err)
			})
		}
	})
})
