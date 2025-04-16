# k8sHealth Kubernetes Deployment

This directory contains the necessary Kubernetes configuration files to deploy the k8sHealth component of monokit in a Kubernetes cluster using Kustomize.

## Included Files

- `kustomization.yaml`: Main Kustomize configuration
- `namespace.yaml`: Creates the monitoring namespace
- `rbac.yaml`: RBAC permissions (ServiceAccount, ClusterRole, ClusterRoleBinding)
- `configmap.yaml`: k8s-specific configuration
- `global-configmap.yaml`: Global monokit configuration
- `cronjob.yaml`: CronJob for periodic monitoring

## Setup and Deployment

### 1. Customize the Configuration

Before deploying, you'll need to customize two configuration files:

1. **k8s-specific configuration**: Edit `configmap.yaml` to include your actual floating IPs and ingress floating IPs:

```yaml
k8s:
  floating_ips:
    - YOUR_FLOATING_IP_1
    - YOUR_FLOATING_IP_2
  ingress_floating_ips:
    - YOUR_INGRESS_IP_1
    - YOUR_INGRESS_IP_2
```

2. **Global monokit configuration**: Edit `global-configmap.yaml` to configure:
   - Alarm settings (Slack webhook URLs)
   - Redmine integration
   - Bot notifications
   - Other global settings

### 2. Configure CronJob Schedule

The configuration uses a CronJob for periodic monitoring, which runs every 30 minutes by default. If you want to change the schedule:

1. Edit `cronjob.yaml`
2. Modify the `schedule` field to your desired cron schedule
   - Example: `"*/15 * * * *"` to run every 15 minutes
   - Example: `"0 * * * *"` to run at the top of every hour

### 3. Deploy Using Kustomize

```bash
# From the kubernetes directory
kubectl apply -k .
```

Or specify the full path:

```bash
kubectl apply -k /path/to/monokit/k8sHealth/kubernetes
```

## About Periodic Monitoring

This configuration uses a CronJob for periodic monitoring which:

- Runs k8sHealth checks on a scheduled basis (default: every 30 minutes)
- Consumes fewer resources than a continuous deployment
- Is ideal for cluster health checks that don't require real-time monitoring
- Creates a new pod for each execution and cleans up automatically

## Customization Options

If you need to make more extensive customizations, you can:

1. **Change the image tag**: Edit the `newTag` field in the `kustomization.yaml` file
2. **Modify the CronJob schedule**: Edit the `schedule` field in the `cronjob.yaml` file  
3. **Add resource limits**: Add resource requests/limits to the container spec in cronjob.yaml
4. **Change failure handling**: Modify the `restartPolicy` or add `failedJobsHistoryLimit` to the CronJob spec

## Deployment Options

There are two ways to run k8sHealth in Kubernetes:

1. **Continuous Monitoring (Deployment)**: Uses a Deployment to run k8sHealth continuously. This is useful if you want real-time monitoring and immediate alerts.

2. **Periodic Monitoring (CronJob)**: Uses a CronJob to run k8sHealth periodically (default: every 30 minutes). This is more resource-efficient but has delayed detection of issues.

By default, the base configuration includes both resources, but the Deployment is active. Use the cronjob overlay to switch to periodic monitoring.

## Permissions

The k8sHealth component requires the following permissions to function properly:

- Read access to Nodes, Pods, and Namespaces
- Read access to cert-manager Certificates
- Access to Pod logs

These permissions are provided via the ClusterRole in the `rbac.yaml` file.

## Troubleshooting

If you encounter issues with the deployment, check the logs of the k8sHealth pod:

```bash
kubectl logs -n monitoring -l app=k8shealth
```

For CronJob deployments, check the logs of the most recent job:

```bash
kubectl logs -n monitoring -l job-name=k8shealth-cronjob-<job-id>
```