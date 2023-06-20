# Troubleshooting

## NAP Scale Up

```sh
kubectl describe pod <pending-pod>
```

```
LAST SEEN   TYPE      REASON          OBJECT                          MESSAGE
12s         Warning   FailedScaleUp   pod/pod-test-5b97f7c978-h9lvl   Node scale up in zones associated with this pod failed: Internal error. Pod is at risk of not being scheduled
```

The root cause could be serial-port-logging-enable being false or auto\_upgrade being set to false.
See: https://cloud.google.com/kubernetes-engine/docs/troubleshooting/troubleshooting-autopilot-clusters#scale-up-failed-serial-port-logging

Solution:


```sh
gcloud compute project-info add-metadata \
    --metadata serial-port-logging-enable=true
```

In case your issue is related to auto\_upgrade ensure your cluster autoprovisioning defaults enable auto upgrade:
```
gcloud container clusters update CLUSTER_NAME \
    --enable-autoprovisioning --enable-autoprovisioning-autorepair \
    --enable-autoprovisioning-autoupgrade
```



Recreate cluster. View errors with more info.

```sh
kubectl describe pod <pending-pod>
```
```
  Normal   TriggeredScaleUp   35s   cluster-autoscaler  pod triggered scale-up: [{https://www.googleapis.com/compute/v1/projects/cnp-demo-dev/zones/us-central1-a/instanceGroups/gke-substratus-nap-n1-standard-4-gpu1-dab8d858-grp 0->1 (max: 1000)}]
  Warning  FailedScaleUp      13s   cluster-autoscaler  Node scale up in zones us-central1-a associated with this pod failed: GCE out of resources. Pod is at risk of not being scheduled.
```
