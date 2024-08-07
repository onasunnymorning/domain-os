# domain-os

Domain (Registry for now) Operating System (DOS)

 # Why?
 The registry backend business operates in a high-volume low-margin setup, therefore a highly automated, robust, adaptable system that is cost efficient to operate is required. 
 It should be flexible in terms of policy, infrastructure and quick to evolve. 

 # How? 
 Offer clear context, a blazing fast deveoper experience and feedback loop and lots of automated testing for safety.
 I've tried to focus on making this system easy to operate with a low overhead. While it is not yet optimized, it has testing, automation, visibility, an open architecture and documentation.
 This way when optimization is needed it will be staightforward as we will have data and visibility to evolve the system quickly in the desired direction.
 The data model aligns with the RFCs and can easily be evolved because its de-coupled from the business logic.

  # What?
  A registry system with core functionality that you can easily integrate into your existing service stack or build on top of

# Running the app
## Components
* persistent storage - db (currently Postgres) to run alongside the app for a quick demo or test deployment
* messaging and event streaming (currently Kafka) to allow de-coupling of services.
* API first (currently Gin) to ensure consisten business logic through any endpoint
* EPP server that can use the API over the network or directly import the business logic as a package
* HTTP EPP client to quickly interface with EPP for visibility and quick feedback
* DNS component that produces a bind compatible zonefile. This can be evolved to be a standalone component that uses events to stay up to date.
* WHOIS/RDAP for compliance, focus here is data protection and DOS/Excessive crawling mitigation
* ICANN reporting that offers complete transparency
* ESCROW import/export functionality for RDE and importing data
* Migration toolkit

## Deployment
The app is containerized and geared towards a kubernetes runtime.
* githun CI pipeline running unit and integration tests + push new images (CD pending)
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

Compose/Tilt up
