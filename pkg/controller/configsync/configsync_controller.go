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
	"bytes"
	"context"
	"flag"
	"io/ioutil"
	"os"
	"path"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const localFileMode os.FileMode = 0444

var localDirFlag string
var configMapNameFlag string
var configMapNamespaceFlag string
var packageLogger = logf.Log.WithName("configsync-controller")

func init() {
	flag.StringVar(&localDirFlag, "output-dir", "", "sync configmap contents to this directory")
	flag.StringVar(&configMapNameFlag, "config-map-name", "", "watch this configmap")
	flag.StringVar(&configMapNamespaceFlag, "config-map-namespace", "", "watch in this namespace")
}

// Add creates a new ConfigSync Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr, localDirFlag), configMapNameFlag, configMapNamespaceFlag)
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, dir string) reconcile.Reconciler {
	return &ReconcileConfigSync{
		Client:   mgr.GetClient(),
		scheme:   mgr.GetScheme(),
		localDir: dir,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
//
// +kubebuilder:rbac:groups=,resources=configmaps,verbs=get;list;watch
func add(mgr manager.Manager, r reconcile.Reconciler, cm, ns string) error {
	// Create a new controller
	c, err := controller.New("configsync-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	nameFilter := func(o v1.Object) bool {
		return o.GetName() == cm && o.GetNamespace() == ns
	}
	// Watch for changes to our exact configmap
	p := predicate.Funcs{
		UpdateFunc:  func(e event.UpdateEvent) bool { return nameFilter(e.MetaOld) },
		CreateFunc:  func(e event.CreateEvent) bool { return nameFilter(e.Meta) },
		DeleteFunc:  func(e event.DeleteEvent) bool { return nameFilter(e.Meta) },
		GenericFunc: func(e event.GenericEvent) bool { return nameFilter(e.Meta) },
	}
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{}, p)
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileConfigSync{}

// ReconcileConfigSync reconciles a ConfigSync object
type ReconcileConfigSync struct {
	client.Client
	scheme   *runtime.Scheme
	localDir string
}

// Reconcile reads the state of a ConfigMap and mirrors its contents as files in a directory
func (r *ReconcileConfigSync) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	log := packageLogger.WithName("reconcile")
	instance := &corev1.ConfigMap{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{Requeue: true}, err
	}

	// { basename: contents }
	localFiles := make(map[string]bool)

	files, err := ioutil.ReadDir(r.localDir)
	if err != nil {
		log.Error(err, "could not read directory", "dirname", r.localDir)
		return reconcile.Result{}, err
	}
	for _, file := range files {
		localFiles[file.Name()] = true
	}

	for cmFile, cmStr := range instance.Data {
		cmContents := []byte(cmStr)
		localContents, err := ioutil.ReadFile(path.Join(r.localDir, cmFile))
		if err != nil || bytes.Compare(localContents, cmContents) != 0 {
			if err = atomicWrite(r.localDir, cmFile, cmContents); err != nil {
				log.Error(err, "failed updating file", "dir", r.localDir, "file", cmFile)
			}
		}
		delete(localFiles, cmFile)
	}
	// any files left in `localFiles` are no longer in the configmap and should be removed
	for fn := range localFiles {
		os.Remove(path.Join(r.localDir, fn))
	}

	return reconcile.Result{}, nil
}

func atomicWrite(dir string, file string, data []byte) error {
	f, err := ioutil.TempFile(dir, "."+file+"*")
	defer os.Remove(f.Name())
	if err != nil {
		return err
	}
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = os.Chmod(f.Name(), localFileMode); err != nil {
		return err
	}
	return os.Rename(f.Name(), path.Join(dir, file))
}
