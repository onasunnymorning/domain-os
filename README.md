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

## Getting up and running
First rename example.env to .env
```bash
mv example.env .env
```
Then edit the .env file to your taste (or leave default for local development)

Next run the following command to start the app
```
BRANCH=latest docker compose --profile essential -f docker-compose.yml up
```

### troubleshooting
if you get this error
```
Error response from daemon: failed to create task for container: failed to create shim task: OCI runtime create failed: runc create failed: unable to start container process: error during container init: error mounting "/host_mnt/Users/gprins/dos/.rabbitmq/enabled_plugins" to rootfs at "/etc/rabbitmq/enabled_plugins": mount /host_mnt/Users/gprins/dos/.rabbitmq/enabled_plugins:/etc/rabbitmq/enabled_plugins (via /proc/self/fd/6), flags: 0x5000: not a directory: unknown: Are you trying to mount a directory onto a file (or vice-versa)? Check if the specified host path exists and is the expected type
```
you should check if you have .rabbitmq/enabled_plugins file containting the following content:
`[rabbitmq_stream, rabbitmq_prometheus].`
