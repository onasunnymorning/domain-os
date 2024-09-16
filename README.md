# domain-os

Domain (Registry for now) Operating System (DOS)

 # Why?
 The registry backend business operates in a high-volume low-margin setup, therefore a highly automated, robust, adaptable system that is cost efficient to operate is required. 
 It should be flexible in terms of policy, infrastructure and quick to evolve and integrate. 

 # How? 
 Offer clear context, a blazing fast developer experience and feedback loop and lots of automated testing for safety.
 I've tried to focus on making this system easy to operate with a low overhead. While it is not yet optimized, it has testing, automation, visibility, and an open architecture and documentation.
 This way, when optimization is needed, it will be staightforward as we will have data and visibility to evolve the system quickly in the desired direction.
 The data model aligns with the RFCs and can easily be evolved, optimized or even be implemented with a different database because its de-coupled from the business logic.

  # What?
  A registry system with core functionality that you can easily integrate into your existing service stack or build on top of

# Running the app
## Components
### INFRA dependencies
* Database (Postgres) to run alongside the app for a quick demo or test deployment
* Messaging (Rabbit MQ) to allow de-coupling of services.
* Registrar Data (IANA and ICANN) to allow for compliance and data integrity
* FX (Foreign Exchange) rates provider to allow for currency conversion
* APM (NewRelic) can be enabled optionally
* Monitoring (Prometheus) to allow for visibility, can be enabled optionally
* Compliance (ICANN RRI and MOSAPI) to allow for compliance and data integrity

### APP components
* Domain layer: Packages that can be imported and contain all of the business logic and allows configurable options. Highly unit tested.
* API (Go-Gin) REST API that can be used to interface with the domain layer. This endpoint is geared towards ADMIN users. It could be used by other componenets, but it might be more efficient to import the domain layer directly.
* EPP client
* EPP server
* DNS component that produces a bind compatible zonefile. This can be evolved to be a standalone component that uses events to stay up to date.
* WHOIS/RDAP for compliance, focus here is data protection and DOS/Excessive crawling mitigation
* ICANN reporting that offers complete transparency
* ESCROW import/export functionality for RDE and importing data
* Migration toolkit

## Deployment
The app is containerized and suggests a kubernetes deployment for production and docker-compose or Tilt for development.
* github CI pipeline running unit and integration tests + push new images (CD pending)
* Helm for templating deployments
* Docker compose or Tilt for a quick feedback look when developing
* Postman integration tests

### Setting up a development environment
Make sure you have the following installed:
* git
* docker
* kubectl
* helm
* tilt (optional if you plan to use docker compose)

Check out the code

Set your ENVARS
This repository contains an `example.env` file that you can use to set your environment variables. 


Compose/Tilt up


# Deploying the app
## Requirements
```
$ cat .eksctl/minimal-cluster.yaml
apiVersion: eksctl.io/v1alpha5
kind: ClusterConfig

metadata:
  name: poc-cluster
  region: us-west-2

nodeGroups:
  - name: ng-1
    instanceType: m5.large
    desiredCapacity: 1
$ eksctl create cluster -f .eksctl/minimal-cluster.yaml

```
