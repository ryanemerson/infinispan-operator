# Existing Approach

## Separate Resource Controllers
The Operator currently utilises multiple controller instances to reconcile Infinispan CR and associated resources.

### Advantages
- Different resource types handled by different controllers
  - Good separation of concerns
  - Reconcile events on specific controllers only triggered for appropriate events, e.g. secret updates for `infinispan_secret_controller`

### Disadvantages
- Faux parallelism
  - Many of the controllers rely on resources created by other controllers
  - The happens-before relationship of resource creation/consumption is less clear with the separation
  - Created resources must also be loaded by dependent controllers 

- Code repetition
  - All controllers have to retrieve the Infinispan CR, ensure it doesn't have it's deletion timestamp set etc 

- Performance
  - Additional load is placed on the api-server as resources must be looked up per controller
  - Multiple controllers react to the same reconcile event

- Redundant Controllers
  - The `hotrod_upgrade_controller` is only required if `spec.Upgrade.Type == HotRodRolling`, however it reconciles all
Infinispan CR updates, loads the Infinispan CR and returns upon realising no action is required. 

# Infinispan Reconcile Workflow
The Infinispan CR reconcile workflow can be broadly split into the following stages:

1. Collect existing resources
   - Infinispan CR instance
   - Existing CR resources, StatefulSet etc
   - User resources, e.g. ConfigMap, Secrets
   - XSite Backups

2. Generate Resource Specs
   - StatefulSet, Services, Secrets and ConfigMaps

3. Manage Upgrades
   - Compare resources from stages 1, 2 and trigger an upgrade if required

4. Manage Runtime
   - Infinispan Status, e.g. ClusterWellFormed
   - ConsoleUrl status
   - Configure Loggers
   - CacheService AutoScale
  
# Pipeline Approach
The stages defined in the [workflow](#infinispan-reconcile-workflow) have a

## Components

### Pipeline