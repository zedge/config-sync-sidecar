/*
Copyright 2019 Zedge, Inc.

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

package configsync

import (
	"fmt"
	"github.com/Flaque/filet" // Ω "github.com/onsi/gomega"
	. "github.com/onsi/gomega"
	"golang.org/x/net/context"
	"io/ioutil"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"os"
	"path"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sort"
	"testing"
	"time"
)

var c client.Client

var expectedRequest = reconcile.Request{NamespacedName: types.NamespacedName{Name: "foo", Namespace: "default"}}

const timeout = time.Second * 5

func TestAtomicWrite(t *testing.T) {
	g := NewGomegaWithT(t)
	tmpDir := filet.TmpDir(t, "")
	fn := "file1.txt"
	g.Expect(atomicWrite(tmpDir, fn, []byte("contents"))).To(Succeed())
	g.Expect(filesInDir(t, tmpDir)).To(Equal([]string{fn}))
}

func TestReconcile(t *testing.T) {
	g := NewGomegaWithT(t)
	cmName := "foo"
	cmNamespace := "default"
	instance := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: cmName, Namespace: cmNamespace},
		Data: map[string]string{
			"file1": "contents1",
		},
	}

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{})
	g.Expect(err).To(Succeed())
	c = mgr.GetClient()

	tmpDir := filet.TmpDir(t, "")
	tmpDirLsFunc := filesInDirFunc(t, tmpDir)
	recFn, requests := SetupTestReconcile(newReconciler(mgr, tmpDir))
	g.Expect(add(mgr, recFn, cmName, cmNamespace)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	// Create the ConfigSync object and expect the Reconcile and Deployment to be created
	g.Expect(c.Create(context.TODO(), instance)).To(Succeed())
	// The instance object may not be a valid object because it might be missing some required fields.
	// Please modify the instance object by adding required fields and then remove the following if statement.
	if apierrors.IsInvalid(err) {
		t.Logf("failed to create object, got an invalid object error: %v", err)
		return
	}
	defer c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	g.Eventually(tmpDirLsFunc, timeout).Should(Equal(mapKeys(instance.Data)))
	g.Expect(allFilesInDirAre0444(tmpDir, mapKeys(instance.Data))).To(Succeed())

	// Add two keys to the configmap and expect the corresponding files to be created
	instance.Data["file2"] = "contents2"
	instance.Data["file3"] = "contents3"
	g.Expect(c.Update(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	g.Eventually(tmpDirLsFunc, timeout).Should(Equal(mapKeys(instance.Data)))
	g.Expect(allFilesInDirAre0444(tmpDir, mapKeys(instance.Data))).To(Succeed())

	// Remove a key from the configmap and expect the corresponding file to be deleted
	delete(instance.Data, "file1")
	g.Expect(c.Update(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout).Should(Receive(Equal(expectedRequest)))
	g.Eventually(tmpDirLsFunc, timeout).Should(Equal(mapKeys(instance.Data)))
	g.Expect(allFilesInDirAre0444(tmpDir, mapKeys(instance.Data))).To(Succeed())
}

func allFilesInDirAre0444(dir string, files []string) error {
	for _, file := range files {
		s, _ := os.Stat(path.Join(dir, file))
		m := s.Mode() & 0777
		if m != localFileMode {
			return fmt.Errorf("%s/%s has mode %#o", dir, file, m)
		}
	}
	return nil
}

func mapKeys(m map[string]string) []string {
	keys := make([]string, len(m))
	i := 0
	for k := range m {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	return keys
}

func filesInDir(t *testing.T, dir string) []string {
	return filesInDirFunc(t, dir)()
}

func filesInDirFunc(t *testing.T, dir string) func() []string {
	return func() []string {
		fileNames := make([]string, 0)
		files, err := ioutil.ReadDir(dir)
		if err != nil {
			t.Logf("could not read directory %s: %v", dir, err)
			return fileNames
		}
		for _, file := range files {
			fileNames = append(fileNames, file.Name())
		}
		sort.Strings(fileNames)
		return fileNames
	}
}
