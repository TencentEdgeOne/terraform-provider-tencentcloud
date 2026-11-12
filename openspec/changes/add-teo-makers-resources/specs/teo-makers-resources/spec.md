## ADDED Requirements

### Requirement: Makers forwarding call
The system SHALL call Makers interfaces through the TEO actions `DescribePagesResources`, `CreatePagesResources`, `ModifyPagesResources` and `DeletePagesResources`, choosing the action by the verb prefix of the interface name, with `ZoneId`, `Interface` (`pages:<name>`) and a JSON string `Payload` at the top level of the request body.

#### Scenario: Interface verb selects the action
- **WHEN** the provider calls `pages:CreatePagesDeployment`
- **THEN** the request SHALL use the action `CreatePagesResources` and decode `Response.Result` as JSON

#### Scenario: Missing resource
- **WHEN** the API returns error code `ResourceNotFound` for a describe call
- **THEN** the resource SHALL be treated as not existing and removed from state on read

### Requirement: Makers project resource
The system SHALL provide `tencentcloud_teo_makers_project` with required `zone_id` (ForceNew) and `name`, optional `area` (ForceNew; `mainland`, `overseas`, `global`), optional `nodejs_version`, `root_directory`, `output_directory`, `build_command`, `install_command`, `framework`, and computed `project_id`, `preset_domain`, `status`, `created_on`. The id SHALL be `zone_id#project_id`.

#### Scenario: Create upload project
- **WHEN** user applies a new project
- **THEN** the system SHALL call `pages:CreatePagesProject` once with `Provider=Upload`, `Channel=Custom`, `Source=terraform`, without retrying

#### Scenario: Rename in place
- **WHEN** only `name` changes
- **THEN** the system SHALL call `pages:ModifyPagesProject` with `Name` and no unchanged fields

#### Scenario: Delete project
- **WHEN** the resource is destroyed
- **THEN** the system SHALL call `pages:DeletePagesProject`, treating `ResourceNotFound` as success

### Requirement: Makers project environment variables resource
The system SHALL provide `tencentcloud_teo_makers_project_env_vars` that owns the complete set of environment variables of a project. Each `env_vars` item has `key`, sensitive `value`, optional `comment` and optional `environments`; a null `environments` means every environment of the project.

#### Scenario: Reconcile rows
- **WHEN** applying the resource
- **THEN** the system SHALL delete remote rows whose key and environment scope are not configured by row `Id`, then create or update configured rows, sending `Id` for rows whose value or comment changed

#### Scenario: Drift detection
- **WHEN** a variable is changed or added outside Terraform
- **THEN** the next plan SHALL show an update for the resource

#### Scenario: Reject values the platform rewrites
- **WHEN** a key, value or comment has leading or trailing whitespace, or `environments` is an empty set, or two items of the same key share an environment
- **THEN** validation SHALL fail before any API call

### Requirement: Makers deployment resource
The system SHALL provide `tencentcloud_teo_makers_deployment` with required `zone_id`, `project_id`, exactly one of `source_dir` and `source_zip`, optional `environment` (default `Production`), `source_hash`, `triggers`, a `timeouts.create` attribute (default 20 minutes), and computed `deployment_id`, `status`, `preview_url`, `project_url`, `created_on`. The id SHALL be `zone_id#project_id#deployment_id`.

#### Scenario: Hash computed during plan
- **WHEN** `source_hash` is not configured
- **THEN** ModifyPlan SHALL hash the files under `source_dir`, skipping `node_modules`, `.edgeone`, `.env`, `.git` and `.DS_Store`, and require replacement when the hash differs from state

#### Scenario: Zip archive source
- **WHEN** `source_zip` is configured
- **THEN** ModifyPlan SHALL reject a path that is not a regular `.zip` file with a zip signature, hash the archive content, and apply SHALL upload the archive to `<TargetPath>/<file name>` and create the deployment with `DistType` `Zip` and that object key as `TempBucketPath`

#### Scenario: Ambiguous source
- **WHEN** both or neither of `source_dir` and `source_zip` are configured
- **THEN** validation SHALL fail

#### Scenario: Imported deployment adopts the hash
- **WHEN** the state has no `source_hash`
- **THEN** the plan SHALL set the computed hash without requiring replacement

#### Scenario: Failed build
- **WHEN** the deployment finishes with a status other than `Success`
- **THEN** apply SHALL fail with the last lines of the build log and the resource SHALL be stored in state so that it is tainted

#### Scenario: Destroy keeps history
- **WHEN** the resource is destroyed
- **THEN** the system SHALL only remove it from state

### Requirement: Makers custom domain resource
The system SHALL provide `tencentcloud_teo_makers_custom_domain` with required `zone_id`, `project_id`, `domain` (all ForceNew), optional `environment` (default `Production`), and computed `status`, `cname`, `current_cname`, `ownership_verification`. The id SHALL be `zone_id#project_id#domain`.

#### Scenario: Change environment in place
- **WHEN** only `environment` changes
- **THEN** the system SHALL call `pages:ModifyPagesZoneCustomDomainEnv`

#### Scenario: Unbind an already removed domain
- **WHEN** `pages:DeletePagesProjectCustomDomains` fails and the domain is no longer bound to the project
- **THEN** the delete SHALL succeed

### Requirement: Makers zones data source
The system SHALL provide `tencentcloud_teo_makers_zones` listing the Makers zones of the account, optionally filtered by exact `zone_name`.
