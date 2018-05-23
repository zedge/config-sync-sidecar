---
title: Kubernetes Config Sync Sidecar
---

This is a service for keeping a local directory in sync with the contents of a config map.
It watches a given config map for changes, and immediately rsyncs the contents of it to
a given directory.
