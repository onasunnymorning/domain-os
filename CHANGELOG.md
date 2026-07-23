<a name="unreleased"></a>
## [0.8.4](https://github.com/onasunnymorning/domain-os/compare/v0.8.3...v0.8.4) (2026-07-23)


### Bug Fixes

* **deps:** bump grpc to v1.82.1 to clear Trivy HIGH finding ([0fa9ddb](https://github.com/onasunnymorning/domain-os/commit/0fa9ddb7d0b34f168c47ee30e8361eac5d17a0b8))
* **escrow:** heartbeat every staging import phase ([727ffe3](https://github.com/onasunnymorning/domain-os/commit/727ffe3a9bf07058b835178c1b60a0b1b42bf304))
* **escrow:** heartbeat every staging import phase ([93af07b](https://github.com/onasunnymorning/domain-os/commit/93af07b9937631e079c719454614ba45080fdf94))
* **frontend:** hoist postcss override to clear high-severity advisory ([1434776](https://github.com/onasunnymorning/domain-os/commit/14347763425e50dee91b8fb5a0822d7d2a55a567))
* **frontend:** override sharp to 0.35.x to clear npm audit gate ([099485c](https://github.com/onasunnymorning/domain-os/commit/099485ce5e160655873b842ef068aae8a1f1817c))
* **frontend:** regenerate lockfile with npm 10 to match CI ([083e9f0](https://github.com/onasunnymorning/domain-os/commit/083e9f01a7dff35917f340217f63f2c3d38b6a19))
* stop redundant registrar sync events and log bulk-created ClIDs ([f957633](https://github.com/onasunnymorning/domain-os/commit/f95763324c4e4da8b3f4d2283d559992b807bb19))
* stop redundant registrar sync events and log bulk-created ClIDs ([e51a2e2](https://github.com/onasunnymorning/domain-os/commit/e51a2e2e79000aceb817cd4cc2fcbe8aa0fbb32a))

## [0.8.3](https://github.com/onasunnymorning/domain-os/compare/v0.8.2...v0.8.3) (2026-07-16)


### Bug Fixes

* harden DB credential handling, surface auth failures, and clean up deploy contract ([19e603d](https://github.com/onasunnymorning/domain-os/commit/19e603d9e006263dffad77d54db0696ae1700226))

## [0.8.2](https://github.com/onasunnymorning/domain-os/compare/v0.8.1...v0.8.2) (2026-07-11)


### Bug Fixes

* **db:** escape credentials when building the postgres DSN ([3f6ec93](https://github.com/onasunnymorning/domain-os/commit/3f6ec93798c4858a11e35488278260f5929e2da2))

## [0.8.1](https://github.com/onasunnymorning/domain-os/compare/v0.8.0...v0.8.1) (2026-07-11)


### Performance Improvements

* **ci:** cross-compile Go images for arm64 and drop BUILD_DATE ([25a014b](https://github.com/onasunnymorning/domain-os/commit/25a014b2e209afe70b7785c309dcd1d430f75a4f))
* **ci:** cross-compile Go images for arm64 and drop BUILD_DATE ([09ce22f](https://github.com/onasunnymorning/domain-os/commit/09ce22f4d8c63477e83efe9337796a8f4cd271f6))

## 0.8.0 (2026-07-11)


### Features

* 197 improve logging middleware ([e6cd926](https://github.com/onasunnymorning/domain-os/commit/e6cd9269430b3c2ba6d8cf67507b4de601e19840))
* add --streaming option to escrow cli tool driven by the StreamingEscrowService ([b9b2808](https://github.com/onasunnymorning/domain-os/commit/b9b28083a137a29421cf8ce499b832ff48be414d))
* add AcceptDate to DomainTransfer and enhance approval logic with tests ([ee1ad92](https://github.com/onasunnymorning/domain-os/commit/ee1ad925a6415eae34199fc629213b29f0ca7bbc))
* add Accordion and Switch components with animations and styling, and update dependencies ([45f7411](https://github.com/onasunnymorning/domain-os/commit/45f7411dd3fb7a42276a40a3903c1c08f9583992))
* add AllowEscrowImport and EnableDNS fields to TLD, implement SetAllowEscrowImport method, and enhance error handling in TLDService ([a7a0084](https://github.com/onasunnymorning/domain-os/commit/a7a0084f5657c69a1c9baf852f5bf58b462547e3))
* add BulkCreate method to ContactService for batch contact creation ([a5e3e00](https://github.com/onasunnymorning/domain-os/commit/a5e3e006b573a52b4f89ddbca5d137e5ab9cc75b))
* add BulkCreate method to DomainService and DomainRepository for bulk domain creation + expose in DomainController as endpoint ([0f91ca8](https://github.com/onasunnymorning/domain-os/commit/0f91ca8e71b43f06c35918c1cdcc311eaa110669))
* add BulkCreate method to HostRepository Interface ([acabd5b](https://github.com/onasunnymorning/domain-os/commit/acabd5bf91a40fc22f13ebb7adecace3199b4043))
* add BulkCreate method to HostService and HostRepository for creating multiple hosts in a single transaction ([5f6e615](https://github.com/onasunnymorning/domain-os/commit/5f6e6155c9a346f4c0e73d49f0dbf528cdf07c68))
* add BulkCreateContacts endpoint to ContactController and implement BulkCreate method in ContactService ([bd1645f](https://github.com/onasunnymorning/domain-os/commit/bd1645f378aeae86fe5c68f90eb92bca044d8ff0))
* add changelog template and configuration for version tracking ([69b6552](https://github.com/onasunnymorning/domain-os/commit/69b655274e1e25e264e5ce6dceae0e9ce021b056))
* Add Clone method to Host and Domain entities for deep copying functionality include zap.logger DomainLifecycleEvents on relevant DomainService functions ([031d465](https://github.com/onasunnymorning/domain-os/commit/031d465e49b4e56caa36a8c811b4ece1fda710bb))
* add Cloud infrastructure page with service links ([d04a432](https://github.com/onasunnymorning/domain-os/commit/d04a432574cf5e89450d8b5b84b90f074e138b1d))
* add CoLogo component and implement a toggleable logo switcher in the Sidebar ([7eb1742](https://github.com/onasunnymorning/domain-os/commit/7eb1742dcd8f59d8b3c62ad9d0a833f9ff76d346))
* add comprehensive EPP server update summary including library migration, rate limiting, Docker containerization, and testing infrastructure ([e36d9f6](https://github.com/onasunnymorning/domain-os/commit/e36d9f612060a04d81d50e6ce23fad380ab2d883))
* add configurable log level for EPP server and update documentation ([8ea5787](https://github.com/onasunnymorning/domain-os/commit/8ea578731844f9f88d18a2f108103df03d92fca0))
* Add correlation ID handling to workflows and activities + lifecycle cli ([410a8eb](https://github.com/onasunnymorning/domain-os/commit/410a8ebd663e19344bf2a5175bdac834d3866798))
* add Count endpoint to NNDNController with filtering options ([930404f](https://github.com/onasunnymorning/domain-os/commit/930404f12b316a9801b0b2436d1ad8ff1ce2c8e6))
* add Count method to GormNNDNRepository for counting NNDNs with filters ([52c947d](https://github.com/onasunnymorning/domain-os/commit/52c947d249fb7a4c4bf7ce460463ac82ee29418f))
* add Count method to NNDNService and NNDNRepository for counting NNDNs with optional filters ([65501d7](https://github.com/onasunnymorning/domain-os/commit/65501d78b00b6ec78f9e7e02abf7b553aa2d30a7))
* add Count method to RegistryOperatorRepository for counting with filters ([be6bb0f](https://github.com/onasunnymorning/domain-os/commit/be6bb0fb35528dc94edfb9809e8cd3243f547c0d))
* Add create and update registrar functionality with form validation ([3cfef60](https://github.com/onasunnymorning/domain-os/commit/3cfef6072804d3865c2071562a62afdfd9ad557b))
* add CreateRegistrar activity and update SyncRegistrarsWorkflow to handle reserved registrars ([5cc1afa](https://github.com/onasunnymorning/domain-os/commit/5cc1afa134329e4af2d62493e05ccbb9d307fea9))
* add CSV export functionality and improve filter UI across TLDs and Registrar pages ([e909f47](https://github.com/onasunnymorning/domain-os/commit/e909f478e56e3a98c9792a6fac16e68a97d5d635))
* add csv-to-sqlite subcommand to escrow cli to build a queryable sqllite database from the csv files produced by the analysis command ([b9b2808](https://github.com/onasunnymorning/domain-os/commit/b9b28083a137a29421cf8ce499b832ff48be414d))
* Add DNS lookup modal and domain status widget, and enhance domain lifecycle display. ([46c6847](https://github.com/onasunnymorning/domain-os/commit/46c684782e083856840a6dabd365f9320086294b))
* Add domain count display to TLDs page with async loading ([8a1db5e](https://github.com/onasunnymorning/domain-os/commit/8a1db5ece39a71f0eb92067d7b31b3de276fd312))
* add domain management hooks, TLD widgets, and UI components for domain quotes and registrar statistics. ([0b113d1](https://github.com/onasunnymorning/domain-os/commit/0b113d1c400b9093b9b31eeb19d6530d250a4309))
* Add DomainLifecycleWidget to visualize domain registration and grace periods, introducing a new Progress UI component. ([f66f392](https://github.com/onasunnymorning/domain-os/commit/f66f3922d3c62ad91cbf7db1a46245ca91883709))
* add domains management page with filtering and pagination ([d6a6ab1](https://github.com/onasunnymorning/domain-os/commit/d6a6ab1589e791ea281c34dd1a69f60c26e85a7c))
* add dynamic resource links and environment variables for external tools ([e08c24e](https://github.com/onasunnymorning/domain-os/commit/e08c24e80d9126bf80222ff840c13d95baa2612f))
* add email field to CreateRegistrarCommand and update related test assertions ([6db5099](https://github.com/onasunnymorning/domain-os/commit/6db509971cb630dc3cad9cc3062729fe1c474bf4))
* add environment variables reference to Cloud page ([5e29e91](https://github.com/onasunnymorning/domain-os/commit/5e29e9110be8a01ad3a7e8fc6bbf3592b939ae19))
* Add Escrow Import functionality with UI updates and backend support ([11aa993](https://github.com/onasunnymorning/domain-os/commit/11aa9930d4c0751e9ef2f22190d781e47601e1d6))
* Add Escrow Imports functionality and UI integration ([22fd940](https://github.com/onasunnymorning/domain-os/commit/22fd9408d7ac5ac58263400f859fab7d9fecf92f))
* add favicon and Alpaca logo, enhance TLD details with active phases ([2bd5e10](https://github.com/onasunnymorning/domain-os/commit/2bd5e10455f5c99b2e0b89735e45a7ea8790f138))
* add filtering capabilities to ListRegistryOperators endpoint ([f4a0814](https://github.com/onasunnymorning/domain-os/commit/f4a08144d9274119213feaf70ae27f384a941a82))
* add filtering options to ListHosts endpoint ([16ac0cd](https://github.com/onasunnymorning/domain-os/commit/16ac0cd0c72bb835089affd6d75ff1ae6bf3194f))
* add ForceRenew method to domain service and controller for unconditional domain renewal ([4848fe9](https://github.com/onasunnymorning/domain-os/commit/4848fe931fb9fdeb0ce5c48dddc4d5515b479675))
* add frontend and registry-operators/count ([08721c9](https://github.com/onasunnymorning/domain-os/commit/08721c967540896dbb587766e557685420a7e43e))
* add GetRegistrarListItems activity and update workflows to use bearer token for API requests ([946a2ae](https://github.com/onasunnymorning/domain-os/commit/946a2aed55a01e1aca06c3667869f26983a4f500))
* add gitleaks scanning job to CI workflow for secret detection ([3cd35eb](https://github.com/onasunnymorning/domain-os/commit/3cd35eb776155c5ff74eb8c703e367881de9eec5))
* add Helm charts for Metabase and Traefik with necessary configurations and resources ([c4928d8](https://github.com/onasunnymorning/domain-os/commit/c4928d896c06c709712899cca9251570200f29a5))
* add Helm charts for various services and remove deprecated files ([0933709](https://github.com/onasunnymorning/domain-os/commit/093370960987cc60272ec705ecf7e2a14d16edcd))
* add hourly synchronization schedule for registrars and update related workflows ([5ebad4d](https://github.com/onasunnymorning/domain-os/commit/5ebad4debb8505f351452a44ebfbb570d3557ab1))
* add IANARegistrarStatusUnknown and validation method, update registrar struct and tests to inclue IANAStatus ([7259661](https://github.com/onasunnymorning/domain-os/commit/72596619ba949876f8320b56d94737a1edf8bf77))
* add ID field to PremiumLabel entity and update related methods for consistency ([d84e783](https://github.com/onasunnymorning/domain-os/commit/d84e7832f0790d3a07e570082ae3ccecb2a640a4))
* add indexing to DropCatch, RenewedYears, and AuthInfo fields in Domain and TLD structs ([28b95ca](https://github.com/onasunnymorning/domain-os/commit/28b95ca230bfa6355dcada6485733f65a6d06dcb))
* add initial data files and implement ICANN registrar CSV handling ([c5dcb7c](https://github.com/onasunnymorning/domain-os/commit/c5dcb7c6a915f6dbec6126d9d460aa8d30cfc6ba))
* Add IsRegistrarAccreditedForTLD method and response structure for accreditation checks ([399e2b2](https://github.com/onasunnymorning/domain-os/commit/399e2b2c730ed2ed7ae87b6da1a65f57023b9379))
* Add IsRegistrarAccreditedForTLD method, check in Register endpoint and expose as endpoint ([172aeda](https://github.com/onasunnymorning/domain-os/commit/172aeda0aae34bb13de33bbbdf604344d3dedcfc))
* add labels to Kubernetes resources and clean up unused configurations ([acad336](https://github.com/onasunnymorning/domain-os/commit/acad33658358b612c0d07af9bdf8ea6ab080dac0))
* add lifecycle event logging for domain operations and introduce TransactionTypeUpdate ([0045221](https://github.com/onasunnymorning/domain-os/commit/0045221052f24fb3fab5addd01550cfb08fa39f8))
* add ListContactsFilter for query parameter construction and implement tests ([21d4a92](https://github.com/onasunnymorning/domain-os/commit/21d4a921cdde22e0263ccf91b63590cafdb71f74))
* add ListHostsFilter for query parameter construction and implement tests ([dbc5188](https://github.com/onasunnymorning/domain-os/commit/dbc518877d5b7c4cad02fc4003dae18db751d74c))
* add ListPremiumLabelsFilter and ListPremiumListsFilter with ToQueryParams method for query string conversion ([c4c3586](https://github.com/onasunnymorning/domain-os/commit/c4c3586d010de5a630dfd8e04ec37c710cfd53d1))
* Add ListRestoredDomains and CountRestoredDomains methods to DomainRepository ([d123d34](https://github.com/onasunnymorning/domain-os/commit/d123d34e39c3893fdd77f2431d8d783686030429))
* Add ListRestoredDomains and CountRestoredDomains methods to DomainRepository Interface ([e76a6ad](https://github.com/onasunnymorning/domain-os/commit/e76a6ad38a3cc3f11559fe548fb8400bbc895d60))
* Add ListRestoredDomains and CountRestoredDomains methods to DomainService and DomainRepository ([3353f6e](https://github.com/onasunnymorning/domain-os/commit/3353f6e219076bf9e658ac020a063fbe9864f148))
* add ListSpec5LabelsFilter struct and corresponding ToQueryParams method with tests ([a108ab6](https://github.com/onasunnymorning/domain-os/commit/a108ab60a2a7d9fa6672768d21b84a158c2a1bba))
* add MIT License and enhance README with context on system requirements ([e73cf24](https://github.com/onasunnymorning/domain-os/commit/e73cf244f922abece6701dc066e1f19a18038f08))
* add ON CONFLICT DO NOTHING to BulkCreate to prevent batch failures on name collisions ([0f37d70](https://github.com/onasunnymorning/domain-os/commit/0f37d7065f2827098c18467991415809ba45e78f))
* Add phases API for managing TLD phases including listing, creating, and deleting phases ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* add PrettyXMLLogger for formatted XML output and enhance greeting display ([fe05001](https://github.com/onasunnymorning/domain-os/commit/fe05001e75229b4cb1a4fca728021b9e9fc90cc6))
* Add PurgeDomain method to DomainService and implement domain purging logic + expose endpoint - working postman tests ([579a9d8](https://github.com/onasunnymorning/domain-os/commit/579a9d89b87797969547534b34075d9834ce7aa7))
* Add quote retrieval functionality to domain check and lifecycle events ([b85a521](https://github.com/onasunnymorning/domain-os/commit/b85a5210b387ea22b74670687a8e38faf4b118f2))
* Add reasoning validation and new run management features ([6d6fd6d](https://github.com/onasunnymorning/domain-os/commit/6d6fd6d86bd05834cecfb7e6f8bf40a05cbc1c54))
* Add Registrar Management page with IANA and System Registrars tabs ([8aaf354](https://github.com/onasunnymorning/domain-os/commit/8aaf35429eb6baa4ebf224e4059f94a9ac9d5348))
* add registrar synchronization workflow and related activities for IANA registrars ([8191484](https://github.com/onasunnymorning/domain-os/commit/819148425932b8e3d3df4ecb22bfc3f4c7c76291))
* add RegistrarLifecycleEvent entity for event tracking and correlation ([fb60852](https://github.com/onasunnymorning/domain-os/commit/fb608526150c99aeb54a42aad25bbbd3c122cb43))
* add render.yaml Blueprint for TEST env ([4926df5](https://github.com/onasunnymorning/domain-os/commit/4926df56531bf265b59af563910e26b67355998e))
* Add RestoreWorkflow and ListRestoredDomains and RenewDomain activity ([6290590](https://github.com/onasunnymorning/domain-os/commit/62905904621c3a205fc2f0dbeaa62cfa2e316766))
* add retagging step for sync worker in GitHub Actions workflow ([21c489e](https://github.com/onasunnymorning/domain-os/commit/21c489e45b25b715752fab27af6e4d1a44bffbee))
* add SetDomainStatus and UnSetDomainStatus activities with corresponding tests ([383c326](https://github.com/onasunnymorning/domain-os/commit/383c326951d3d423f7e89d52b93756966caacca9))
* add SetStatus and UnSetStatus methods to DomainService for managing domain statuses ([ee50d4f](https://github.com/onasunnymorning/domain-os/commit/ee50d4f46663414989eb7ebb94ba0932a1752f5c))
* add SetTLDStatus and DeleteTLDStatus methods to TLDController for managing AllowEscrowImport status ([c0a7cd2](https://github.com/onasunnymorning/domain-os/commit/c0a7cd250b45d7d3b9d821361ab29129288f4311))
* Add Suspense for lazy loading in Domains and TLDs pages; update ESLint config and domain types for stricter type checks ([35d9ee5](https://github.com/onasunnymorning/domain-os/commit/35d9ee5c1d25f7edc2948c04cd6981dd16fd546d))
* add Temporal stack and domain lifecycle worker to Docker configurations; enhance IANA registrars handling in activities and tests ([ff18504](https://github.com/onasunnymorning/domain-os/commit/ff18504f6a94676df54e7016f6fa2b6c154cdf4c))
* add Temporal UI link to registrar sync workflow response and update docker-compose for UI URL ([601ba1c](https://github.com/onasunnymorning/domain-os/commit/601ba1c56034c94d6f0baef42e95aa9047fe9c03))
* add testing setup with Vitest and Testing Library ([10d8f2a](https://github.com/onasunnymorning/domain-os/commit/10d8f2a8b59bec7da2c6a2c6cf4e13e768459a37))
* add tests for filter extraction from context in premium and NNDN controllers ([6d59ca5](https://github.com/onasunnymorning/domain-os/commit/6d59ca536d859ff4892ed838775db51550658cea))
* Add TLDBadges component to display TLDs associated with a registry ID ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* add TLDs field to RegistryOperator and RyID field to TLD entity ([696688b](https://github.com/onasunnymorning/domain-os/commit/696688bcf5cd838fe4f5130ee6a5f26da2d8f394))
* add ToQueryParams method to ListRegistrarsFilter for converting filters to query string ([9a7f00c](https://github.com/onasunnymorning/domain-os/commit/9a7f00c062b179facb6e986011c8d881f8ddf32a))
* add unified worker deployment for TEST ([269f32a](https://github.com/onasunnymorning/domain-os/commit/269f32aa5b4de3b8aa6904c8f493f7179fe87d17))
* Add utility functions for date formatting and phase status checks ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* Add validation error for invalid domain label in phase and refactor domain check functionality ([7914653](https://github.com/onasunnymorning/domain-os/commit/7914653f0d6f21bc02f5959d9edf47ce8666fb0d))
* aedit json tags on ListItemResult ([e655800](https://github.com/onasunnymorning/domain-os/commit/e6558006cd818f0c0b54815f380ee50f08598d93))
* align frontend phase overlap validation with backend semantics and improve user feedback ([b3671eb](https://github.com/onasunnymorning/domain-os/commit/b3671ebb329e650ef4d21a157f2929d3f91fde79))
* Configure CORS for local development and set frontend API environment variables in Tiltfile. ([a13abd7](https://github.com/onasunnymorning/domain-os/commit/a13abd76711a20f6119db02de0be1974d3d09406))
* consolidate shell scripts into a comprehensive Makefile for improved maintainability and command discoverability ([b62402c](https://github.com/onasunnymorning/domain-os/commit/b62402c0e4bdd939ec2e4426a9762ab42688c0fc))
* convert csv analysis to sqlite ([cc970f5](https://github.com/onasunnymorning/domain-os/commit/cc970f5d3a98e1d1f0bfda1d17fed4c3c3a9bbce))
* Create a reusable Calendar component with custom styling and functionality ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* Create RadioGroup component for selecting options with custom styling ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* Define phase types and structures in TypeScript for better type safety ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* Develop Checkbox component using Radix UI for better accessibility ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* Develop Sheet component for sliding panels with customizable content ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* enable heartbeat-based liveness for escrow import activities ([9071418](https://github.com/onasunnymorning/domain-os/commit/9071418db6fd399369498d2fde67eafadc29c862))
* enhance CreateRegistrarCommand from IANARegistrar with GurID and RdapBaseURL, add validation for GurID, and update tests ([b63d20d](https://github.com/onasunnymorning/domain-os/commit/b63d20dd42dc82d9e34b01d6922ef8848d6355ff))
* enhance dashboard with event tracking, reorganize workflow UI, and refactor domain services and infrastructure components ([277a5f5](https://github.com/onasunnymorning/domain-os/commit/277a5f579c8252287ecebd7e12a5a270d58e6975))
* enhance domain detail page with formatted date displays and links for TLDs and registrars ([05e0d3b](https://github.com/onasunnymorning/domain-os/commit/05e0d3b861acbc80eac5efef01c31137dec4603e))
* Enhance domain lifecycle event logging with correlation ID and price points ([97ddca2](https://github.com/onasunnymorning/domain-os/commit/97ddca2349b497c5c7edb06fc7febbc284fd101d))
* enhance domain lifecycle management with restore schedule and activity updates ([ef99aa3](https://github.com/onasunnymorning/domain-os/commit/ef99aa3354aa339f6bc897b4669ec0c54966a5f5))
* enhance domain transfer functionality with comprehensive checks and new transfer management ([10e1274](https://github.com/onasunnymorning/domain-os/commit/10e127468bff90b94dbbfbbf2ca80c8f43a4f075))
* Enhance escrow import workflow with out-of-band event storage and improved error handling ([3f999ad](https://github.com/onasunnymorning/domain-os/commit/3f999ad12ba2ef6ccd52f8041b07ae0098b433aa))
* enhance GATimeline and PhaseCard components with improved padding and shadow effects for better visual clarity ([5deed39](https://github.com/onasunnymorning/domain-os/commit/5deed393841a16ec1e5bbe6af4b4bb2f0f40a245))
* enhance home page with domain count and create links for domains, TLDs, and registrars ([84731c7](https://github.com/onasunnymorning/domain-os/commit/84731c73138c8167c5da271e1d9919c381360484))
* enhance IANA status display in registrar detail page ([05c6da3](https://github.com/onasunnymorning/domain-os/commit/05c6da3e70bf6100f3f3520a09e5c8bd2463f263))
* Enhance import functionality by adding domain and host status mapping from SQLite ([87d5ae9](https://github.com/onasunnymorning/domain-os/commit/87d5ae9d3a44517cac2000dbc2f9ab0faacec5b2))
* enhance ListContacts method with filtering options and add corresponding tests ([ecef7a7](https://github.com/onasunnymorning/domain-os/commit/ecef7a74cb034ce1ed8dd28b72184631357a6f8d))
* enhance MOSAPI client with measurement details retrieval and logging; add new endpoint for available measurement IDs ([e5a8c39](https://github.com/onasunnymorning/domain-os/commit/e5a8c39260a4f05ce3410ec07e9df12c9dcd8b1f))
* enhance phase components with improved date handling and UI updates ([39a0b5b](https://github.com/onasunnymorning/domain-os/commit/39a0b5b90785d9399c36a279ff6e4dfc61527671))
* enhance phase timeline and card components with improved styling and status indicators ([6caf787](https://github.com/onasunnymorning/domain-os/commit/6caf78773772d1c59202a1845db5bcb649cdefc8))
* enhance PhaseConfigDiff and PhaseDetailDrawer to support detailed pricing and fee comparisons ([6921cf7](https://github.com/onasunnymorning/domain-os/commit/6921cf719f464da0d8b95a677e88c041319905cc))
* enhance PhaseCreateWizard and PhaseDetailDrawer with improved date handling and time display ([517df55](https://github.com/onasunnymorning/domain-os/commit/517df5561426c3435f7934e525dbf1ca42d0a8f7))
* Enhance PhaseDetailDrawer to display empty states for pricing and fees sections ([d49e6c0](https://github.com/onasunnymorning/domain-os/commit/d49e6c0a3a186944933ba0ab751c01acdda8c98b))
* enhance PhaseDetailDrawer with full phase data fetching and improved pricing/fees display ([3795313](https://github.com/onasunnymorning/domain-os/commit/37953137a12feccc7d8934c9d1c51063283cdbe6))
* enhance README with detailed commands and usage examples for escrow tool ([12d9880](https://github.com/onasunnymorning/domain-os/commit/12d98806e1c3566aeec7e09274a77aa6a5370127))
* enhance registrar sync workflow with detailed change reporting, state diffing, and bulk status updates ([6b47a2c](https://github.com/onasunnymorning/domain-os/commit/6b47a2c59e174fb976c439066090cbb3c1b371b1))
* enhance registrar synchronization by adding creation logic for new IANA registrars and improving ClID generation ([3f38b3c](https://github.com/onasunnymorning/domain-os/commit/3f38b3cc82f77962fe5445ccc45f53b9b6551ba1))
* env var registry with AST-based drift detection ([0e26ec5](https://github.com/onasunnymorning/domain-os/commit/0e26ec581ac38228561268f4e3bf3c1f333be167))
* env-driven DB and CORS config, frontend Dockerfile for Render deploy ([79a3fcf](https://github.com/onasunnymorning/domain-os/commit/79a3fcfc2c22c8b369599b69790295874cad4f2f))
* escrow analysis and import workflow ([782ff74](https://github.com/onasunnymorning/domain-os/commit/782ff747e7c63a0c0c20cabfa5b990eaa0fecaba))
* escrow import workflows and UI ([92cbaac](https://github.com/onasunnymorning/domain-os/commit/92cbaac626e04fb727bdb3951f31cbce73676d42))
* Implement add/remove fee functionality in PhaseDetailDrawer with API integration ([9b6bdc5](https://github.com/onasunnymorning/domain-os/commit/9b6bdc5dabe2b7f1cb75273f8fa37ffd018a3ec0))
* Implement agent navigation feature with auto-navigation and action buttons ([a440161](https://github.com/onasunnymorning/domain-os/commit/a440161c42e7c4717066b4860f42724926babb98))
* implement Ask Alpaca agent UI components and backend service integration while cleaning up documentation. ([3dbd824](https://github.com/onasunnymorning/domain-os/commit/3dbd8243de67a9887ba318d7b21bbc33474665b8))
* implement bulk create contacts functionality and improve error handling ([6d45f04](https://github.com/onasunnymorning/domain-os/commit/6d45f041517cebc3465d8d2276cd81699b35db14))
* Implement bulk import functionality for contacts, hosts, and domains via admin API ([9767f43](https://github.com/onasunnymorning/domain-os/commit/9767f433fd008e4bbbce4d85a6b5234af749351a))
* implement bulk registrar creation and add chunk size option for import ([037cf50](https://github.com/onasunnymorning/domain-os/commit/037cf509f03a2ef7dd229b4eb26195c1377b8dce))
* implement BulkCreate method in ContactRepository for batch contact creation + add corresponding tests ([a6e8f4e](https://github.com/onasunnymorning/domain-os/commit/a6e8f4efc19700421846f094223b91674a42dca2))
* implement BulkCreate on HostReposiotry and add gofakeit package for testing ([dd977dc](https://github.com/onasunnymorning/domain-os/commit/dd977dc62d58073abdc6925237973acaea773e6a))
* implement comprehensive workflow orchestration system with new API endpoints, frontend management dashboard, and registry infrastructure ([fc12be4](https://github.com/onasunnymorning/domain-os/commit/fc12be479e2cb76c0e2e3000355fc7be4366faa7))
* Implement contact import functionality and related database schema updates ([be36e78](https://github.com/onasunnymorning/domain-os/commit/be36e785e56bb6b20104a09255b5cc7849a848b8))
* implement cursor pagination and filtering for ListRegistryOperators ([4c391b8](https://github.com/onasunnymorning/domain-os/commit/4c391b87099d503d62b8186ed1d6478b81b6e32d))
* implement daily synchronization schedule for registrars and update import command ([546681c](https://github.com/onasunnymorning/domain-os/commit/546681caf12752524881384e1e793ee7c6ff6995))
* implement DeepCopy methods and RegistrarService Lifecycle logging ([dffb9d8](https://github.com/onasunnymorning/domain-os/commit/dffb9d84a03350cd43ca837636b510ba41a0e307))
* implement EPP login and logout command handling with XML responses ([b31a500](https://github.com/onasunnymorning/domain-os/commit/b31a500ce4204df44e9d83d820e0c836cc9861aa))
* implement event-driven updates for Phase and TLD lifecycle events and introduce new event relay and pruning workflows. ([5b11087](https://github.com/onasunnymorning/domain-os/commit/5b11087094e2e8c143592cb2a4975de8216a7f14))
* implement filrtering on ListPremiumList repository ([cd3319c](https://github.com/onasunnymorning/domain-os/commit/cd3319ca3d1e0a878222515f73f51a03c4aa2409))
* implement filtering in ListNNDNs method with NameLike, TldEquals, ReasonEquals, and ReasonLike options ([5aa92f0](https://github.com/onasunnymorning/domain-os/commit/5aa92f04e17773717abcf6b41ed9ec24da96be07))
* implement formalized bucket storage configuration, S3 client separation, and automated deployment contract generation ([00e3de0](https://github.com/onasunnymorning/domain-os/commit/00e3de01c26f12714434b99df44d56dac30e326e))
* Implement GetQuote functionality in DomainService and add corresponding REST endpoint under /domains ([6692409](https://github.com/onasunnymorning/domain-os/commit/66924092b4c5077dd5e07ecddde899275f04b1ee))
* implement global search functionality with command palette integration ([8cb9e4a](https://github.com/onasunnymorning/domain-os/commit/8cb9e4ac01caf1d1154ed049f338ad0dd69cc34d))
* Implement hooks for fetching and managing phases with React Query ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* implement IANA status management for registrars, including API endpoint and workflow updates ([fd20814](https://github.com/onasunnymorning/domain-os/commit/fd208149a4ff318eac8520fe9927e2a140945cfc))
* implement import command for ICANN and IANA registrars with initial README ([67504d1](https://github.com/onasunnymorning/domain-os/commit/67504d1db4106b365d90adbf610523b541e94882))
* implement inclusive start, exclusive end semantics for phase timing and update related tests ([7940b1a](https://github.com/onasunnymorning/domain-os/commit/7940b1acfef3df79f3eb7c24dd9ad53409951764))
* Implement ListRestoredDomains activity and update /domains/restored response structure ([3ceffd5](https://github.com/onasunnymorning/domain-os/commit/3ceffd5f66b437fbb4857ce8afef9897e2ba3a6d))
* Implement ListRestoredDomains and CountRestoredDomains endpoints in DomainController ([5373522](https://github.com/onasunnymorning/domain-os/commit/5373522fe28d96b0aab5cbfe8062aed12def1a82))
* implement MCP server support and vendor required Go SDK dependencies ([fdc488e](https://github.com/onasunnymorning/domain-os/commit/fdc488eaf7f1b8f5766e19313731604a81b02b59))
* implement MCP server transport support, add automated release management, and introduce Agent Alpaca documentation and build versioning. ([d3c6f33](https://github.com/onasunnymorning/domain-os/commit/d3c6f3382a71d062f14cf1c4f97919fd805a4c62))
* implement MOSAPI client methods for querying available measurement days, months, and years; add response structures ([8cb22b0](https://github.com/onasunnymorning/domain-os/commit/8cb22b055fb0b13524b1d76e2a6e2bd9767df9af))
* Implement NNDN import and improve phase ending UI, DNSSEC algorithm display, and temporary file handling. ([6ea4eaa](https://github.com/onasunnymorning/domain-os/commit/6ea4eaaa5ebcb8798aa065c8b5da4967389a7881))
* Implement Phase Timeline component with categorized phases and detail drawer ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* Implement policy and price editing workflows in PhaseDetailDrawer ([9fb9be2](https://github.com/onasunnymorning/domain-os/commit/9fb9be22acb1b03a02befc3ba200bce5dc202130))
* Implement Popover component for displaying contextual information ([fa4cb10](https://github.com/onasunnymorning/domain-os/commit/fa4cb100b923e9fd58cbc00fd5bc3778b41fd09d))
* implement PostHog analytics for user events, add batch domain processing workflows, and include database utility scripts ([4f06463](https://github.com/onasunnymorning/domain-os/commit/4f06463dffc85c6bf1466cd7b2b1af4cfe0ef66b))
* implement PrettyPrint method for MeasurementDetailsResponse to enhance output formatting ([fde850a](https://github.com/onasunnymorning/domain-os/commit/fde850aa19a7efcb460e86db3cd3ee8c02596d86))
* implement registrar accreditation management, including API and UI updates for adding and de-accrediting registrars ([7c638a5](https://github.com/onasunnymorning/domain-os/commit/7c638a5f1acdcd32a2241c3dc6cdb2d5f27b5c01))
* implement registrar sync workflow and add dialog for workflow status ([6f2a665](https://github.com/onasunnymorning/domain-os/commit/6f2a665f21aca2843c7baaa57d9a077166e86fac))
* implement registrar synchronization with IANA repository and add status update logic ([91ef71c](https://github.com/onasunnymorning/domain-os/commit/91ef71cdcb3fa85682819449337ff33a2f8a4452))
* implement registrar synchronization workflow and related activities ([9936d8e](https://github.com/onasunnymorning/domain-os/commit/9936d8e6654f054ddb5680e0674418f0acdaeecb))
* implement server-side event search with cursor-based pagination and frontend integration ([c8ea0c8](https://github.com/onasunnymorning/domain-os/commit/c8ea0c897df27a0b688caaf8b9518704b7307f37))
* Implement shared `DataTable`, `ListPageLayout`, `SearchFilter`, and `DeleteConfirmDialog` components, and refactor the registry operators and TLDs pages to utilize ([fa2b150](https://github.com/onasunnymorning/domain-os/commit/fa2b150a12d89201fa5413d5becbbade7aef43bc))
* implement snapshotting, Spec5 sweep, and registrar seeding workflows while deprecating legacy schedules and Metabase deployment assets. ([55172b4](https://github.com/onasunnymorning/domain-os/commit/55172b42518926bc5ec5e886ec5904c1175061e0))
* Implement TLD deletion with confirmation dialog and enhance domain import to include domain status information. ([efa0659](https://github.com/onasunnymorning/domain-os/commit/efa0659bb24ca103208e9feb47ace3f42fa2eda9))
* Implement TLD management features ([644761a](https://github.com/onasunnymorning/domain-os/commit/644761add842a9b306da7c02e19280047e4ffec1))
* implement tombstone archival, zone slaving, and serial drift detection systems with supporting frontend interfaces and documentation. ([3622ea9](https://github.com/onasunnymorning/domain-os/commit/3622ea9ed223bf7c772f49c8f983245ca0680d8a))
* implement tombstone management, zone slaving services, and serial drift detection workflows with associated frontend interfaces. ([0a18eca](https://github.com/onasunnymorning/domain-os/commit/0a18eca3abd5a1c85362a6b251fee44b3504769c))
* implement UnSetDomainStatus Activity for removing domain statuses and add corresponding tests ([f9fac2c](https://github.com/onasunnymorning/domain-os/commit/f9fac2c0a641e6f9119d11800a4c66c5813f7f27))
* Improve domain detail page by adding registrar links, a copy button, and refining domain age presentation. ([b178ba9](https://github.com/onasunnymorning/domain-os/commit/b178ba993f447d9451c1e37abe628a3dd3ce9da3))
* improve phase drawer handling and URL management in PhaseTimeline component ([2d68e3c](https://github.com/onasunnymorning/domain-os/commit/2d68e3c0b40acf207a26de6cd43178dcf8c9bbc7))
* infra update ([c365cac](https://github.com/onasunnymorning/domain-os/commit/c365cac04878bd8058dfba176b1e87022cfe1f4e))
* integrate Auth0 authentication into DnssecPage and secure API requests with dynamic access tokens ([221887a](https://github.com/onasunnymorning/domain-os/commit/221887a3b326dc806509ba9e97d98375f27971c9))
* integrate PostHog analytics and refactor RestoreWorkflow to support batch processing with real-time progress monitoring. ([aa3c790](https://github.com/onasunnymorning/domain-os/commit/aa3c79036c917fad14328c99de2ed15c2b3c33ec))
* Introduce correlation ID in activities and workflows + cli (calling apps) ([18bd2e4](https://github.com/onasunnymorning/domain-os/commit/18bd2e4367abe4204831eee69fc7b5f31cd827a5))
* Introduce DNSSEC visualization and new domain, TLD, and registrar widgets. ([3289a1f](https://github.com/onasunnymorning/domain-os/commit/3289a1fcdfb0926d3048f65c1e2668cb03312e61))
* introduce ListItemsQuery for improved query handling ([5d546ea](https://github.com/onasunnymorning/domain-os/commit/5d546ea2b22e77d1fea1dd15f6b2faaa95020e0e))
* introduce RegistrarListItem type and update related methods for improved registrar listing ([2a8b994](https://github.com/onasunnymorning/domain-os/commit/2a8b9949e0e255aa4d7aea8c298580dcea3c9d32))
* introduce RoidService interface and its mock implementation for testing ([b8ad8e3](https://github.com/onasunnymorning/domain-os/commit/b8ad8e334ff241be69ab661da2f18348c497a96a))
* make Reason field required in CreateNNDNCommand and set default value in FromRDENNDN method ([cf77685](https://github.com/onasunnymorning/domain-os/commit/cf77685094c5df18d4ca9aec46ff5089c40f281d))
* make Reason field required in CreateNNDNCommand and set default… ([d9120ec](https://github.com/onasunnymorning/domain-os/commit/d9120ec7f63fdc2d4177fc4ecd1ef9ba61210660))
* migrate storage configuration to STORAGE_* environment variables with legacy MINIO_* fallback and add IAM authentication support ([804387c](https://github.com/onasunnymorning/domain-os/commit/804387c053159e9d038b3555e829eea7180ae142))
* Normalize domain and host names by trimming trailing dots and whitespace in AddHostToDomain functions ([107a0cf](https://github.com/onasunnymorning/domain-os/commit/107a0cfe451a9516d2d111896619febe3102a9d6))
* redesign TLD and Registry Operator detail pages for improved document-like presentation and user experience ([fbcfe7f](https://github.com/onasunnymorning/domain-os/commit/fbcfe7fabfcbaaf90eafff2e4516ce9761164edf))
* refactor domain counting logic to accept filters and improve query handling ([4c30c11](https://github.com/onasunnymorning/domain-os/commit/4c30c1157a60358629b9e5bb5521cd0c9d878f93))
* refactor domain status commands to use ToggleDomainStatusCommand and update related activities and tests ([29782a6](https://github.com/onasunnymorning/domain-os/commit/29782a63080c676b6807c25cfbde3c544fa4eaea))
* refactor List method in RegistrarService and RegistrarRepository to use ListItemsQuery for improved filtering ([0553566](https://github.com/onasunnymorning/domain-os/commit/0553566f4e16d4e83383dfcf83767869de70e420))
* refactor ListContacts and ListHosts methods to improve filter handling and pagination logic ([d284e2c](https://github.com/onasunnymorning/domain-os/commit/d284e2cc9f7c976507c0b7d5a30261467225f131))
* refactor ListContacts method to use ListItemsQuery for improved pagination and filtering ([3d7741e](https://github.com/onasunnymorning/domain-os/commit/3d7741e9a975d2daabd460d7dcb8b774410628ae))
* refactor ListHosts method to use ListItemsQuery for improved parameter handling ([3a91f34](https://github.com/onasunnymorning/domain-os/commit/3a91f348ee79b9b7e9d0f332cebfd9e68262f617))
* refactor ListLabels method to use ListItemsQuery for improved filtering and pagination ([e523fe6](https://github.com/onasunnymorning/domain-os/commit/e523fe6eb8cd67f8c79dc7d10ee0fc3ccad1170e))
* Refactor metadata display for consistency in Registry Operators and TLD detail pages ([08629e7](https://github.com/onasunnymorning/domain-os/commit/08629e73d62417a2898c10c5ccc261d8d9f4f519))
* refactor Spec5 service and repository to use ListItemsQuery for pagination and filtering ([ec57e52](https://github.com/onasunnymorning/domain-os/commit/ec57e5271d77310d5c2d5f3208e8e9636e5e3508))
* refactor TLD listing to use ListTldQuery for improved pagination and filtering ([6288f0d](https://github.com/onasunnymorning/domain-os/commit/6288f0dec43754377357b5ce9e18d4ae36e718af))
* remove rabbitMQ and event generation in controllers ([a6bd9c1](https://github.com/onasunnymorning/domain-os/commit/a6bd9c10062618a3effc557d441469e316723f2e))
* replace OpenAI SDK with Anthropic SDK and implement askg orchestration components ([b504581](https://github.com/onasunnymorning/domain-os/commit/b5045810ca29f3d1c05b4055ce00bf170c123c56))
* restore workflow ([d085965](https://github.com/onasunnymorning/domain-os/commit/d085965abceb59bc8b1e59a15012cf96c4354272))
* small change to logging + vendor ([ad4b54b](https://github.com/onasunnymorning/domain-os/commit/ad4b54b5cf8b29e73476008add095659b1213fab))
* tld cleanup workflow, UI and refresh Tilt config ([9d12abb](https://github.com/onasunnymorning/domain-os/commit/9d12abb58ea4eb64289ee4132641ab61d112c343))
* unify escrow import workflow and enhance frontend UI with new dashboard components and progress tracking ([02a8067](https://github.com/onasunnymorning/domain-os/commit/02a80679d0a691a7e899f2ecf6bad6c16ad7142a))
* update app version to 0.2.2 and add flag to ignore errors during import analysis ([5a2b21d](https://github.com/onasunnymorning/domain-os/commit/5a2b21d361732c988b9e5ac9cc2647940192944f))
* Update application branding to "Alpaca Names" in layout and header components ([f84f2a2](https://github.com/onasunnymorning/domain-os/commit/f84f2a2067954575ec1511f2d437fe491feb66db))
* update Count method to return int64 instead of int for NNDNs and add filtering support in controller ([3e83ac9](https://github.com/onasunnymorning/domain-os/commit/3e83ac92a26a0642ce85702f4d40eb59610c1165))
* update CountTLDs method to accept filters for improved counting functionality ([2e523b3](https://github.com/onasunnymorning/domain-os/commit/2e523b33d234a4a4f6f80d5510a95b0cb793757b))
* update CreateTLD method to return TLD entity and improve error handling ([c96087a](https://github.com/onasunnymorning/domain-os/commit/c96087a80092d16b5294142396e886100c529fe6))
* Update Dockerfile to create a non-root user for running the application ([1b47215](https://github.com/onasunnymorning/domain-os/commit/1b47215fb2ff59dcf42164e73be1aa9270e24a7c))
* Update Dockerfiles to create non-root user for application execution ([e556b3e](https://github.com/onasunnymorning/domain-os/commit/e556b3eefa1f6cd9458053a4e064ece5907352ec))
* update DomainRoiD type to RoidType for improved type safety in domain transfers ([e25f952](https://github.com/onasunnymorning/domain-os/commit/e25f95231333636e88799ea2a0913803a791b71c))
* update functionality in activities and restoreWorkflow ([4137f1e](https://github.com/onasunnymorning/domain-os/commit/4137f1e6f1644e640f876441052ccd06cdf200f9))
* update Go base image to 1.24.1 and add Helm chart for workers ([f1398ec](https://github.com/onasunnymorning/domain-os/commit/f1398ec8a1b7343409fb353051a2dc2a649ee0ed))
* update IANA registrars retrieval to include base URL and API token, and switch logger to production mode ([03c8d7a](https://github.com/onasunnymorning/domain-os/commit/03c8d7ac0470ff72cb57eb9bc8c84689402157f9))
* update JWT token handling to use environment variable for API_TOKEN ([0164723](https://github.com/onasunnymorning/domain-os/commit/0164723518a5a9ddb295d1a092c74b8deccae66d))
* update ListTLDs method to return cursor for pagination support ([3c152bc](https://github.com/onasunnymorning/domain-os/commit/3c152bc49cd9a32262af3463d1b1a927252cfa17))
* update Makefile help menu and enhance stack management commands ([f6cc338](https://github.com/onasunnymorning/domain-os/commit/f6cc33869d3b2e618427b9c5f4d64f75cb99db9b))
* update MosapiClientConfig for certificate-based authentication and OTE environment ([2c2391a](https://github.com/onasunnymorning/domain-os/commit/2c2391aa5d1765706636cefad2e38e59524249a4))
* update query parameter names in ListContacts filter for consistency and add ClidEquals filter ([5886495](https://github.com/onasunnymorning/domain-os/commit/5886495796436f29ae4804051a72e358fdbe3085))
* update README and environment configuration for improved setup instructions and clarity ([9e31d4d](https://github.com/onasunnymorning/domain-os/commit/9e31d4d5eed991c6e5ba098edb14234a27f2afd8))
* update registrar creation to return entity and enhance API documentation ([f7b17cc](https://github.com/onasunnymorning/domain-os/commit/f7b17cc364342cc28fc63c161de85bf60948bc5b))
* update RenewDomain function to support forced renewal option and add corresponding tests ([598552a](https://github.com/onasunnymorning/domain-os/commit/598552ae672fdcc746ac226a317dd506be9b99e1))
* update RestoreWorkflow to handle PendingRestore domains and improve logging ([2a6f2f7](https://github.com/onasunnymorning/domain-os/commit/2a6f2f7606adea999b5fba93edcfc9e6048cb387))
* Update RestoreWorkflow to use RenewDomainCommand for domain renewal ([c048e17](https://github.com/onasunnymorning/domain-os/commit/c048e17211d4e8d732e14be7e9b27a717d536111))
* update success response types in accreditation endpoints ([ae808e4](https://github.com/onasunnymorning/domain-os/commit/ae808e4b5515529097ae23f4eeeaf09d3f7b7d3c))
* update TLD creation to include RyID and adjust related tests ([6df8f04](https://github.com/onasunnymorning/domain-os/commit/6df8f04be0611a2d496e1eb2b1c817542a575dec))
* Upgrade Docker build-push action to v6 in CI workflows ([9ba85b4](https://github.com/onasunnymorning/domain-os/commit/9ba85b437485489c45034a0e97c755465cf420ba))


### Bug Fixes

* add ADMIN_TOKEN environment variable to merge workflow ([127d7c0](https://github.com/onasunnymorning/domain-os/commit/127d7c0e6a3a97a83f496f5946f01c23bee60e15))
* add ADMIN_TOKEN to environment variables in CI workflow ([9490375](https://github.com/onasunnymorning/domain-os/commit/9490375f64ef26bd9ebd75422a23cecb1ca4a8eb))
* add APIKey to all Temporal client configs for Cloud auth ([ab0373d](https://github.com/onasunnymorning/domain-os/commit/ab0373d02b2ddf235fde98e844d948f5d32d54ba))
* add defaults to BASEURL init() to prevent 'http://:' URLs ([05c968f](https://github.com/onasunnymorning/domain-os/commit/05c968fc2670057336b5f76d13ef1b9cf8749014))
* Add log import to measurement query files for improved logging capabilities ([a428676](https://github.com/onasunnymorning/domain-os/commit/a42867670510e7eb2ac9a35c15a042d3f36bdc5d))
* add NEXT_PUBLIC_API_TOKEN as build arg for client-side auth ([d2aad4a](https://github.com/onasunnymorning/domain-os/commit/d2aad4a388e5288f8adb383c31f1ba4208bf1e46))
* add nil checks for rateLimiter to prevent nil pointer dereference in connection handling ([6d6ae27](https://github.com/onasunnymorning/domain-os/commit/6d6ae2757fdf081c0edbd62cac0465d096af1bf3))
* **ci:** unbreak image builds and clear Trivy HIGH/CRITICAL findings ([f3dcf44](https://github.com/onasunnymorning/domain-os/commit/f3dcf448ad08b9851040939dd824cf7f0c84314d))
* Clarify Restore method documentation in Domain entity to improve understanding of its functionality and error handling ([5d31ce8](https://github.com/onasunnymorning/domain-os/commit/5d31ce8fde1f743ae9f355817b238cf397679011))
* **contract:** declare secrets explicitly, derive used_by, and add whois service ([84c0e7c](https://github.com/onasunnymorning/domain-os/commit/84c0e7c8dfde1c9f2dc438fde696da02231884ad))
* correct BulkCreate method in ContactService to avoid duplicates ([8260fa1](https://github.com/onasunnymorning/domain-os/commit/8260fa1c323517cdad185c1cb9416273bdc9586f))
* correct formatting issue in GetCreateCommands function ([fc35289](https://github.com/onasunnymorning/domain-os/commit/fc3528967a4415ad3d8796c62e804bfd999fd57a))
* correct JSON tag for AllowEscrowImport field in TLD struct ([65344e8](https://github.com/onasunnymorning/domain-os/commit/65344e8833f1f38cdec92308d3a815895fcc91cf))
* correct type for PageSize in PaginationMetaData and update test to include filter ([7aa1764](https://github.com/onasunnymorning/domain-os/commit/7aa176426d4127320b367dd849fbc3f0e206234c))
* correct typo in log message for retrieving existing registrars ([516fc25](https://github.com/onasunnymorning/domain-os/commit/516fc256a3e4ccf9aee3ac2ee42730ac90101409))
* disable npm cache in GitHub Actions to resolve jsdom compatibility issue ([8962d60](https://github.com/onasunnymorning/domain-os/commit/8962d6018473c98d2b0b5375dfdcb8f1b04cc6d7))
* downgrade jsdom version to 25.0.1 for compatibility ([9a004be](https://github.com/onasunnymorning/domain-os/commit/9a004be1bd0eb54f0082961ef75dac85a1843156))
* enable TLS for Temporal Cloud API Key auth ([160c760](https://github.com/onasunnymorning/domain-os/commit/160c760c94446e3ef0241fb68a91f561d1b723fb))
* enhance error handling for RoId filters and validation in domain repository ([9d635dd](https://github.com/onasunnymorning/domain-os/commit/9d635ddf459af105a08f6b1281c8cec59920e011))
* enhance error message for ICANN XML Spec5 Registry unmarshalling to indicate potential site maintenance or format changes ([9afd0ec](https://github.com/onasunnymorning/domain-os/commit/9afd0ec1a659613bedbf4cb0a63ef8a0c98f1f63))
* enhance integration test command with Postman requirements and cleanup steps ([ab3f4ec](https://github.com/onasunnymorning/domain-os/commit/ab3f4ec11a87132fe32c5f25a2128aee82f7602d))
* handle empty analysis JSON in registrar mapping import ([bf41f07](https://github.com/onasunnymorning/domain-os/commit/bf41f071ba3c333138a6a6322c2d519663bc3ccb))
* Handle ErrPhaseNotFound in GetQuote method to return a 400 status code ([c4ce2dd](https://github.com/onasunnymorning/domain-os/commit/c4ce2dde4e5719411752df502c2515816ee4bbd3))
* handle failed file moves in BuildStagingDatabase ([f07c3ee](https://github.com/onasunnymorning/domain-os/commit/f07c3ee02707a8684462258d8a9eeec958411eb6))
* implement Fix-CNIC01- try and correct status error and return warning ([7f009c6](https://github.com/onasunnymorning/domain-os/commit/7f009c6dfc6b21967178374748f9670568176397))
* improve error message for ICANN XML Spec5 Registry unmarshalling to include specific URL for better clarity ([0627e66](https://github.com/onasunnymorning/domain-os/commit/0627e66117cca19988a8af9c52b7b3e3e8161036))
* modify GetDomainByName call in PurgeDomain to include hosts so we can dissasociate the hosts if needed ([93c1596](https://github.com/onasunnymorning/domain-os/commit/93c15968022324d0ad8b704cde7338a5854adc0a))
* pin vite to v5.x and @vitejs/plugin-react to v4.x for vitest compatibility ([4ea0d0b](https://github.com/onasunnymorning/domain-os/commit/4ea0d0b69b5b47ea90c43481ddbdb3d62add3567))
* prevent infinite redirect loop in protected route during Auth0 callback processing and add test coverage ([2dab672](https://github.com/onasunnymorning/domain-os/commit/2dab67236034500ce9c2c115fee5f1fb43dc5983))
* prevent rendering of deleted domains and registrars in event feed links ([704134b](https://github.com/onasunnymorning/domain-os/commit/704134b3d6867b009db677c2e5ccbbe97aad5d61))
* set TLS ServerName for SNI on go-pg connections to Neon ([20c5a8e](https://github.com/onasunnymorning/domain-os/commit/20c5a8e00c9f9d35a30fd7bd6ce2336127b6e455))
* standardize ClID field naming to 'Clid' across filters and queries ([cc02fe2](https://github.com/onasunnymorning/domain-os/commit/cc02fe27bd4a6619983b5273fc25c5b4975cced2))
* standardize query parameter naming to 'pagesize' across controllers ([6bf910f](https://github.com/onasunnymorning/domain-os/commit/6bf910fa7522ecbd72c0793baa68ad8a78758e37))
* standardize tag naming for NNDN endpoint documentation ([de928ba](https://github.com/onasunnymorning/domain-os/commit/de928ba241e3f9c7bb9b97cbac37671215e69cd9))
* strip unsupported query params from DATABASE_URL for go-pg ([ee6e688](https://github.com/onasunnymorning/domain-os/commit/ee6e68886cae8813ac6ab2fdc9f28c5936a7c7bb))
* support API_URL in escrow activities' buildAdminAPIURL ([4ca2e17](https://github.com/onasunnymorning/domain-os/commit/4ca2e175c21fd94d1a86961ae76b72a1fc2b7ea2))
* support DATABASE_URL in DirectDBImporter ([b1394c1](https://github.com/onasunnymorning/domain-os/commit/b1394c16e188d0a74ca0272bbeecc2c2bfe650d4))
* update file path for ICANN registrars CSV in SyncRegistrarsWorkflow ([30893be](https://github.com/onasunnymorning/domain-os/commit/30893be0fa2b9e8b6c285b0f96a873012bfc7961))
* update IANA Registrars Tab to display "Last updated: Never" when count is 0 ([ef12659](https://github.com/onasunnymorning/domain-os/commit/ef126591e58bb4f8428ab33d6deeb20d60eb7ac0))
* Update IDNA label validation to use Registration.ToUnicode for i… ([5635622](https://github.com/onasunnymorning/domain-os/commit/5635622c046818312a53fe693ed07185f5dc397e))
* Update IDNA label validation to use Registration.ToUnicode for improved accuracy in DNS ([2ef326f](https://github.com/onasunnymorning/domain-os/commit/2ef326f2c64d3f632f4a4e1448bac79c83f78433))
* update integration test environment cleanup and docker-compose network configuration ([3a25919](https://github.com/onasunnymorning/domain-os/commit/3a259190aa3258ebab78a437ef644c91fd1a4b55))
* update isPhaseCurrent logic to correctly handle ongoing phases ([d9c8a6b](https://github.com/onasunnymorning/domain-os/commit/d9c8a6be29efd5e95aae33202fbf13f7aaec653a))
* update new registrar tests to expect default autoresenew to true and improve test container cleanup ([2d7cb90](https://github.com/onasunnymorning/domain-os/commit/2d7cb9092a59ffe10a26080ad926ce2fb118957b))
* update queries to use ILIKE for case-insensitive matching in List and Count methods ([94936ae](https://github.com/onasunnymorning/domain-os/commit/94936ae05ff1f27314fede276a9fec576d96012d))
* Update quote endpoint to /domains/quote and adjust tags accordingly - working integration tests ([d7e65eb](https://github.com/onasunnymorning/domain-os/commit/d7e65eb8c49b049315ca064c7152f2044ae9d0a0))
* update RyID validation schema to enforce character limits and ASCII compliance ([7c6518b](https://github.com/onasunnymorning/domain-os/commit/7c6518bb5f20474334f50b04027c48ab5505eda6))
* update section header for clarity on running the app with Docker Compose ([c9063be](https://github.com/onasunnymorning/domain-os/commit/c9063be0c98edb1bc04799d3e50d200fc88454cd))
* update welcome message to reflect correct application context ([a3a246a](https://github.com/onasunnymorning/domain-os/commit/a3a246aea73ff99b2e3368a12046552ae056b797))
* upgrade net lib + tidy and vendor ([f7a9cf5](https://github.com/onasunnymorning/domain-os/commit/f7a9cf5e0dcef21296225353fd29a16154429bd7))
* use Auth0 TokenManager in escrow adminAPIGet ([ba1e4ff](https://github.com/onasunnymorning/domain-os/commit/ba1e4ff9bbdefe2f72f83187d6ff14f1175ee6fe))
* use buildAdminAPIURL() in ResolveRegistrars ([518c085](https://github.com/onasunnymorning/domain-os/commit/518c085b99c80de788316a6a9f2287d290b493ea))


### Performance Improvements

* stream S3 → gzip → XML decoder without temp files ([f87fd65](https://github.com/onasunnymorning/domain-os/commit/f87fd6583114f99363b96d0b4da5bdaae75f729a))


### Miscellaneous Chores

* release 0.8.0 ([610b658](https://github.com/onasunnymorning/domain-os/commit/610b65883f9979bb6f0156771fd74045f0998a41))

## [Unreleased]


<a name="v0.6.1"></a>
## v0.6.1 - 2025-04-12
### Chore
- update comment for Phase entity to include reference link
- move startTestDBServer script for PostgreSQL database setup and cleanup scripts
- update example.env for database host and event stream configuration
- update ToQueryParams method documentation for various filter types to clarify conversion to query string
- remove redundant build and push steps for Event Consumer and EPP Client API in CI workflows
- bump crypto lib to 0.35.0 for https://scout.docker.com/vulnerabilities/id/CVE-2025-22869/org/geapex?s=golang&n=crypto&ns=golang.org%2Fx&t=golang&vr=%3C0.35.0
- cleanup
- update Alpine base image version to 3.21.3 in multiple Dockerfiles
- vendor
- vendor
- update Go module dependencies and versioning
- Enhance DomainRGPStatus with detailed comments for clarity on time periods
- Refactor quotes into domainService and remove quoteService and quoteController
- Refactor expiry-loop.go to use environment variables for API host and port
- Update default value for days in ListExpiringDomains method
- Set RenewedYears when creating a domain and in grace delete support
- Add support for output format selection in DNSController and TLDController
- Add Kafka environment variables for CI integration
- Add Kafka configuration to CI workflow

### Docs
- enhance BulkCreateContacts method documentation for clarity on error handling

### Feat
- add changelog template and configuration for version tracking
- refactor Spec5 service and repository to use ListItemsQuery for pagination and filtering
- add ListSpec5LabelsFilter struct and corresponding ToQueryParams method with tests refactor: rename TestToQueryParams to TestToNNDNQueryParams for clarity
- aedit json tags on ListItemResult
- implement filrtering on ListPremiumList repository
- add tests for filter extraction from context in premium and NNDN controllers
- add ID field to PremiumLabel entity and update related methods for consistency
- refactor ListLabels method to use ListItemsQuery for improved filtering and pagination
- add ListPremiumLabelsFilter and ListPremiumListsFilter with ToQueryParams method for query string conversion
- add ToQueryParams method to ListRegistrarsFilter for converting filters to query string
- refactor List method in RegistrarService and RegistrarRepository to use ListItemsQuery for improved filtering
- add Count endpoint to NNDNController with filtering options
- update Count method to return int64 instead of int for NNDNs and add filtering support in controller
- add Count method to NNDNService and NNDNRepository for counting NNDNs with optional filters
- add Count method to GormNNDNRepository for counting NNDNs with filters
- implement filtering in ListNNDNs method with NameLike, TldEquals, ReasonEquals, and ReasonLike options
- add Count method to RegistryOperatorRepository for counting with filters
- update CountTLDs method to accept filters for improved counting functionality
- refactor domain counting logic to accept filters and improve query handling
- update query parameter names in ListContacts filter for consistency and add ClidEquals filter
- refactor ListContacts and ListHosts methods to improve filter handling and pagination logic
- enhance ListContacts method with filtering options and add corresponding tests
- refactor ListContacts method to use ListItemsQuery for improved pagination and filtering
- add ListContactsFilter for query parameter construction and implement tests
- add filtering options to ListHosts endpoint
- add filtering capabilities to ListRegistryOperators endpoint
- add ListHostsFilter for query parameter construction and implement tests
- implement cursor pagination and filtering for ListRegistryOperators
- add indexing to DropCatch, RenewedYears, and AuthInfo fields in Domain and TLD structs
- update ListTLDs method to return cursor for pagination support
- refactor ListHosts method to use ListItemsQuery for improved parameter handling
- introduce ListItemsQuery for improved query handling
- remove rabbitMQ and event generation in controllers
- refactor TLD listing to use ListTldQuery for improved pagination and filtering
- update Go base image to 1.24.1 and add Helm chart for workers
- add labels to Kubernetes resources and clean up unused configurations
- infra update
- add Helm charts for various services and remove deprecated files
- small change to logging + vendor
- update app version to 0.2.2 and add flag to ignore errors during import analysis
- add AcceptDate to DomainTransfer and enhance approval logic with tests
- update README and environment configuration for improved setup instructions and clarity
- update DomainRoiD type to RoidType for improved type safety in domain transfers
- enhance domain transfer functionality with comprehensive checks and new transfer management
- add Helm charts for Metabase and Traefik with necessary configurations and resources
- implement bulk create contacts functionality and improve error handling
- add BulkCreate method to HostService and HostRepository for creating multiple hosts in a single transaction
- introduce RoidService interface and its mock implementation for testing
- add BulkCreate method to HostRepository Interface
- implement BulkCreate on HostReposiotry and add gofakeit package for testing
- add BulkCreateContacts endpoint to ContactController and implement BulkCreate method in ContactService
- add BulkCreate method to ContactService for batch contact creation
- implement BulkCreate method in ContactRepository for batch contact creation + add corresponding tests
- add BulkCreate method to DomainService and DomainRepository for bulk domain creation + expose in DomainController as endpoint
- add gitleaks scanning job to CI workflow for secret detection
- add lifecycle event logging for domain operations and introduce TransactionTypeUpdate
- implement DeepCopy methods and RegistrarService Lifecycle logging
- enhance CreateRegistrarCommand from IANARegistrar with GurID and RdapBaseURL, add validation for GurID, and update tests
- add IANARegistrarStatusUnknown and validation method, update registrar struct and tests to inclue IANAStatus
- update registrar creation to return entity and enhance API documentation
- add CreateRegistrar activity and update SyncRegistrarsWorkflow to handle reserved registrars
- add GetRegistrarListItems activity and update workflows to use bearer token for API requests
- update IANA registrars retrieval to include base URL and API token, and switch logger to production mode
- implement daily synchronization schedule for registrars and update import command
- add hourly synchronization schedule for registrars and update related workflows
- add registrar synchronization workflow and related activities for IANA registrars
- add RegistrarLifecycleEvent entity for event tracking and correlation
- implement bulk registrar creation and add chunk size option for import
- enhance registrar synchronization by adding creation logic for new IANA registrars and improving ClID generation
- implement registrar synchronization with IANA repository and add status update logic
- add email field to CreateRegistrarCommand and update related test assertions
- introduce RegistrarListItem type and update related methods for improved registrar listing
- add retagging step for sync worker in GitHub Actions workflow
- implement import command for ICANN and IANA registrars with initial README
- add initial data files and implement ICANN registrar CSV handling
- add SetTLDStatus and DeleteTLDStatus methods to TLDController for managing AllowEscrowImport status
- add AllowEscrowImport and EnableDNS fields to TLD, implement SetAllowEscrowImport method, and enhance error handling in TLDService
- make Reason field required in CreateNNDNCommand and set default value in FromRDENNDN method
- update CreateTLD method to return TLD entity and improve error handling
- update TLD creation to include RyID and adjust related tests
- add TLDs field to RegistryOperator and RyID field to TLD entity
- refactor domain status commands to use ToggleDomainStatusCommand and update related activities and tests
- add SetDomainStatus and UnSetDomainStatus activities with corresponding tests
- implement UnSetDomainStatus Activity for removing domain statuses and add corresponding tests
- add SetStatus and UnSetStatus methods to DomainService for managing domain statuses
- update RestoreWorkflow to handle PendingRestore domains and improve logging
- update RenewDomain function to support forced renewal option and add corresponding tests
- add ForceRenew method to domain service and controller for unconditional domain renewal
- enhance domain lifecycle management with restore schedule and activity updates
- update functionality in activities and restoreWorkflow
- Update RestoreWorkflow to use RenewDomainCommand for domain renewal
- Add RestoreWorkflow and ListRestoredDomains and RenewDomain activity
- Implement ListRestoredDomains activity and update /domains/restored response structure
- Implement ListRestoredDomains and CountRestoredDomains endpoints in DomainController
- Add ListRestoredDomains and CountRestoredDomains methods to DomainService and DomainRepository
- Add ListRestoredDomains and CountRestoredDomains methods to DomainRepository Interface
- Add ListRestoredDomains and CountRestoredDomains methods to DomainRepository
- Upgrade Docker build-push action to v6 in CI workflows
- Update Dockerfiles to create non-root user for application execution
- Update Dockerfile to create a non-root user for running the application
- Add PurgeDomain method to DomainService and implement domain purging logic + expose endpoint - working postman tests
- Add Clone method to Host and Domain entities for deep copying functionality include zap.logger DomainLifecycleEvents on relevant DomainService functions
- Implement GetQuote functionality in DomainService and add corresponding REST endpoint under /domains
- Add validation error for invalid domain label in phase and refactor domain check functionality
- Add quote retrieval functionality to domain check and lifecycle events
- Enhance domain lifecycle event logging with correlation ID and price points
- Introduce correlation ID in activities and workflows + cli (calling apps)
- Add correlation ID handling to workflows and activities + lifecycle cli
- Add IsRegistrarAccreditedForTLD method and response structure for accreditation checks
- Add support for client ID in ListExpiringDomains method
- Add DockerHub credentials for CI integration

### Fix
- upgrade net lib + tidy and vendor
- standardize tag naming for NNDN endpoint documentation
- standardize ClID field naming to 'Clid' across filters and queries
- standardize query parameter naming to 'pagesize' across controllers
- correct type for PageSize in PaginationMetaData and update test to include filter
- update queries to use ILIKE for case-insensitive matching in List and Count methods
- improve error message for ICANN XML Spec5 Registry unmarshalling to include specific URL for better clarity
- enhance error message for ICANN XML Spec5 Registry unmarshalling to indicate potential site maintenance or format changes
- enhance error handling for RoId filters and validation in domain repository
- implement Fix-CNIC01- try and correct status error and return warning
- correct BulkCreate method in ContactService to avoid duplicates
- update file path for ICANN registrars CSV in SyncRegistrarsWorkflow
- correct typo in log message for retrieving existing registrars
- correct formatting issue in GetCreateCommands function
- correct JSON tag for AllowEscrowImport field in TLD struct
- modify GetDomainByName call in PurgeDomain to include hosts so we can dissasociate the hosts if needed
- Handle ErrPhaseNotFound in GetQuote method to return a 400 status code
- Update quote endpoint to /domains/quote and adjust tags accordingly - working integration tests
- Update IDNA label validation to use Registration.ToUnicode for improved accuracy in DNS
- Clarify Restore method documentation in Domain entity to improve understanding of its functionality and error handling

### Refactor
- modify setDomainFilters to return modified query and handle errors in ListDomains and Count methods
- update setTldFilters to return modified query and handle errors in List and Count methods
- extract filter logic into separate function for improved readability and maintainability
-  pagination and filtering for ListNNDNs with ListItemsQuery struct
- remove indexing from DropCatch, RenewedYears, AllowEscrowImport, and EnableDNS fields in Domain and TLD structs
- remove redundant parameter documentation in DomainService methods
- streamline metadata handling in ListHosts and ListTLDs methods
- improve cursor pagination and filtering in ListDomains and ListTlds methods
- update docker-compose and repository methods for improved health checks and pagination
- rename ListTldQuery to ListTldsQuery for consistency and clarity
- rename SetAllowEscrowImport to ToggleAllowEscrowImport for clarity
- optimize BulkCreate method in HostService for better host slice handling
- update log messages in EscrowImportController for clarity on host duplication
- rename CreateDomain method to Create for consistency across domain repository + add BulkCreate to Domain repository
- rename CreateDomain method to Create for consistency
- remove CreateRegistrarCommandResult and update API documentation to reflect changes in response type + working postman tests
- rename ICANN and IANA related packages and files for clarity
- Update endpoint for PurgeDomain method to use Purge endpoint instead of Admin Delete
- Enhance logging for domain lifecycle events with detailed messages
- Enhance DomainLifeCycleEvent struct with additional CorrelationID field and improved comments for clarity
- Replace string TransactionType with TransactionType type for improved type safety and clarity

### Test
- enhance ListTLD tests with filtering capabilities and pagination adjustments
- improve expiry date validation in domain transfer tests
- Enhance Clone tests for Domain and Host entities with comprehensive test cases

### Pull Requests
- Merge pull request [#273](https://github.com/onasunnymorning/domain-os/issues/273) from onasunnymorning/244-implement-filter-functionality-on-entity-list-endpoints
- Merge pull request [#268](https://github.com/onasunnymorning/domain-os/issues/268) from onasunnymorning/267-security-update
- Merge pull request [#264](https://github.com/onasunnymorning/domain-os/issues/264) from onasunnymorning/infra-update
- Merge pull request [#262](https://github.com/onasunnymorning/domain-os/issues/262) from onasunnymorning/260-rework-escrow-import-to-use-batch-endpoints
- Merge pull request [#259](https://github.com/onasunnymorning/domain-os/issues/259) from onasunnymorning/255-bulk-create-endpoints
- Merge pull request [#257](https://github.com/onasunnymorning/domain-os/issues/257) from onasunnymorning/241-lost-commits
- Merge pull request [#254](https://github.com/onasunnymorning/domain-os/issues/254) from onasunnymorning/241-create-a-workflow-for-registrar-management
- Merge pull request [#252](https://github.com/onasunnymorning/domain-os/issues/252) from onasunnymorning/251-tld-status-flags
- Merge pull request [#250](https://github.com/onasunnymorning/domain-os/issues/250) from onasunnymorning/248-make-nndnreason-mandatory
- Merge pull request [#249](https://github.com/onasunnymorning/domain-os/issues/249) from onasunnymorning/85-link-tld-to-registryoperator
- Merge pull request [#246](https://github.com/onasunnymorning/domain-os/issues/246) from onasunnymorning/229-restoreworkflow---create-workflow-and-schedule-for-lifecycle-events
- Merge pull request [#232](https://github.com/onasunnymorning/domain-os/issues/232) from onasunnymorning/197-improve-logging-middleware
- Merge pull request [#224](https://github.com/onasunnymorning/domain-os/issues/224) from onasunnymorning/220-spike-idn-implementation
- Merge pull request [#222](https://github.com/onasunnymorning/domain-os/issues/222) from onasunnymorning/221-ensure-accreditation-is-present-in-register-endpoint
- Merge pull request [#219](https://github.com/onasunnymorning/domain-os/issues/219) from onasunnymorning/175-allow-thick-and-thin-registry-models
- Merge pull request [#217](https://github.com/onasunnymorning/domain-os/issues/217) from onasunnymorning/184-reorganize-api-documentation
- Merge pull request [#214](https://github.com/onasunnymorning/domain-os/issues/214) from onasunnymorning/213-deal-with-failed-renewals-and-expire-activities
- Merge pull request [#209](https://github.com/onasunnymorning/domain-os/issues/209) from onasunnymorning/208-purging-a-domain-with-hosts-fails-on-fk-constraint
- Merge pull request [#207](https://github.com/onasunnymorning/domain-os/issues/207) from onasunnymorning/204-create-a-workflow-to-update-fx-every-day
- Merge pull request [#206](https://github.com/onasunnymorning/domain-os/issues/206) from onasunnymorning/203-patch-domainexpire
- Merge pull request [#202](https://github.com/onasunnymorning/domain-os/issues/202) from onasunnymorning/201-re-work-expiryloop-to-use-canautorenew-endpoint-to-make-a-choice
- Merge pull request [#200](https://github.com/onasunnymorning/domain-os/issues/200) from onasunnymorning/198-expiryloop-and-purgeloop-testing-with-real-data
- Merge pull request [#195](https://github.com/onasunnymorning/domain-os/issues/195) from onasunnymorning/191-deploy-expiry-and-purge-workers
- Merge pull request [#192](https://github.com/onasunnymorning/domain-os/issues/192) from onasunnymorning/183-create-workflow-definitions-for-expiry-and-purge
- Merge pull request [#187](https://github.com/onasunnymorning/domain-os/issues/187) from onasunnymorning/186-upgrade-packages-to-fix-docker-scout-cves
- Merge pull request [#182](https://github.com/onasunnymorning/domain-os/issues/182) from onasunnymorning/180-the-expiry-loop
- Merge pull request [#174](https://github.com/onasunnymorning/domain-os/issues/174) from onasunnymorning/172-add-https-support
- Merge pull request [#171](https://github.com/onasunnymorning/domain-os/issues/171) from onasunnymorning/167-add-whois-endpoint-to-admin-api
- Merge pull request [#168](https://github.com/onasunnymorning/domain-os/issues/168) from onasunnymorning/165-autorenew
- Merge pull request [#164](https://github.com/onasunnymorning/domain-os/issues/164) from onasunnymorning/157-seeding-for-a-devdemos-system
- Merge pull request [#150](https://github.com/onasunnymorning/domain-os/issues/150) from onasunnymorning/149-add-count-function-to-tld-endpoint
- Merge pull request [#148](https://github.com/onasunnymorning/domain-os/issues/148) from onasunnymorning/147-fix-bug-in-docs
- Merge pull request [#145](https://github.com/onasunnymorning/domain-os/issues/145) from onasunnymorning/142-create-facility-to-add-rr-to-the-apex-zone
- Merge pull request [#144](https://github.com/onasunnymorning/domain-os/issues/144) from onasunnymorning/143-add-integration-tests-for-ns-and-glue-endpoints
- Merge pull request [#141](https://github.com/onasunnymorning/domain-os/issues/141) from onasunnymorning/139-host---address-association-can-create-duplicates
- Merge pull request [#140](https://github.com/onasunnymorning/domain-os/issues/140) from onasunnymorning/138-create-ns-record-endpoint
- Merge pull request [#137](https://github.com/onasunnymorning/domain-os/issues/137) from onasunnymorning/135-create-helm-deployment-of-existing-stack
- Merge pull request [#134](https://github.com/onasunnymorning/domain-os/issues/134) from onasunnymorning/133-fix-label-validator-to-allow-double-dash-except-in-pos-34
- Merge pull request [#131](https://github.com/onasunnymorning/domain-os/issues/131) from onasunnymorning/small-tweaks-messaging
- Merge pull request [#130](https://github.com/onasunnymorning/domain-os/issues/130) from onasunnymorning/118-create-messaging-infrastructure
- Merge pull request [#128](https://github.com/onasunnymorning/domain-os/issues/128) from onasunnymorning/125-escrow-import-is-not-linking-hosts-to-domains-and-therefore-the-inactive-flag-is-not-set
- Merge pull request [#126](https://github.com/onasunnymorning/domain-os/issues/126) from onasunnymorning/120-epp-server
- Merge pull request [#124](https://github.com/onasunnymorning/domain-os/issues/124) from onasunnymorning/122-dev-postgres-container---enable-ssl
- Merge pull request [#123](https://github.com/onasunnymorning/domain-os/issues/123) from onasunnymorning/119-vendor-code
- Merge pull request [#121](https://github.com/onasunnymorning/domain-os/issues/121) from onasunnymorning/helm
- Merge pull request [#116](https://github.com/onasunnymorning/domain-os/issues/116) from onasunnymorning/re-organize-repo
- Merge pull request [#115](https://github.com/onasunnymorning/domain-os/issues/115) from onasunnymorning/114-epp-client
- Merge pull request [#110](https://github.com/onasunnymorning/domain-os/issues/110) from onasunnymorning/108-pricepoint-calculation-logic
- Merge pull request [#109](https://github.com/onasunnymorning/domain-os/issues/109) from onasunnymorning/105-fx-stack--include-ctx
- Merge pull request [#106](https://github.com/onasunnymorning/domain-os/issues/106) from onasunnymorning/104-include-grandfathering-logic-in-price-check
- Merge pull request [#103](https://github.com/onasunnymorning/domain-os/issues/103) from onasunnymorning/101-pricecalulator
- Merge pull request [#102](https://github.com/onasunnymorning/domain-os/issues/102) from onasunnymorning/94-nndn-improvements
- Merge pull request [#100](https://github.com/onasunnymorning/domain-os/issues/100) from onasunnymorning/99-fix-uri-for-domain-renew-restore-endpoints
- Merge pull request [#98](https://github.com/onasunnymorning/domain-os/issues/98) from onasunnymorning/97-upgrade-go-version-for-security
- Merge pull request [#96](https://github.com/onasunnymorning/domain-os/issues/96) from onasunnymorning/77-ensure-price-and-fee-amounts-are-positive-numbers
- Merge pull request [#95](https://github.com/onasunnymorning/domain-os/issues/95) from onasunnymorning/68-spike-dropcatching
- Merge pull request [#93](https://github.com/onasunnymorning/domain-os/issues/93) from onasunnymorning/92-create-repository-to-access-fx-api
- Merge pull request [#91](https://github.com/onasunnymorning/domain-os/issues/91) from onasunnymorning/67-spike-grandfathering
- Merge pull request [#89](https://github.com/onasunnymorning/domain-os/issues/89) from onasunnymorning/87-create-registerdomain-renewdomain-entity-functions
- Merge pull request [#86](https://github.com/onasunnymorning/domain-os/issues/86) from onasunnymorning/78-errors-package-or-githubcompkgerrors
- Merge pull request [#80](https://github.com/onasunnymorning/domain-os/issues/80) from onasunnymorning/65-create-a-domain-price-endpoint
- Merge pull request [#79](https://github.com/onasunnymorning/domain-os/issues/79) from onasunnymorning/64-premium-price-implementation
- Merge pull request [#74](https://github.com/onasunnymorning/domain-os/issues/74) from onasunnymorning/63-create-accreditation-functionality
- Merge pull request [#73](https://github.com/onasunnymorning/domain-os/issues/73) from onasunnymorning/72-mosapi-client-library
- Merge pull request [#59](https://github.com/onasunnymorning/domain-os/issues/59) from onasunnymorning/46-create-an-escrow-framework-that-allows-importing-a-large-valid-rde-escrow-xml-file
- Merge pull request [#50](https://github.com/onasunnymorning/domain-os/issues/50) from onasunnymorning/43-make-phase-and-fee--price-names-slugs
- Merge pull request [#49](https://github.com/onasunnymorning/domain-os/issues/49) from onasunnymorning/44-increase-testing-on-phase-repository-to-and-from-functions
- Merge pull request [#48](https://github.com/onasunnymorning/domain-os/issues/48) from onasunnymorning/35-pass-down-ctx-from-tld-controller-all-the-way-to-the-repo
- Merge pull request [#47](https://github.com/onasunnymorning/domain-os/issues/47) from onasunnymorning/32-verify-all-input-timetime-objects-are-in-utc-on-existing-entities
- Merge pull request [#45](https://github.com/onasunnymorning/domain-os/issues/45) from onasunnymorning/30-create-price-and-fee-service-controller-and-endpoints
- Merge pull request [#42](https://github.com/onasunnymorning/domain-os/issues/42) from onasunnymorning/40-prevent-delete-tld-if-there-are-active-phases
- Merge pull request [#41](https://github.com/onasunnymorning/domain-os/issues/41) from onasunnymorning/29-create-phase-service-controller-and-endpoints
- Merge pull request [#39](https://github.com/onasunnymorning/domain-os/issues/39) from onasunnymorning/tests-cover-fix
- Merge pull request [#34](https://github.com/onasunnymorning/domain-os/issues/34) from onasunnymorning/26-create-repository-for-phase-and-sub-entities
- Merge pull request [#33](https://github.com/onasunnymorning/domain-os/issues/33) from onasunnymorning/25-link-phases-to-tld-as-gaphase-and-launchphase
- Merge pull request [#28](https://github.com/onasunnymorning/domain-os/issues/28) from onasunnymorning/24-add-phase-entity
- Merge pull request [#27](https://github.com/onasunnymorning/domain-os/issues/27) from onasunnymorning/23-add-fee-and-price-entities
- Merge pull request [#22](https://github.com/onasunnymorning/domain-os/issues/22) from onasunnymorning/18-admin-api-domain-endpoints
- Merge pull request [#17](https://github.com/onasunnymorning/domain-os/issues/17) from onasunnymorning/APOS-206_HOSTS
- Merge pull request [#16](https://github.com/onasunnymorning/domain-os/issues/16) from onasunnymorning/APOS-228_Postman-tests
- Merge pull request [#14](https://github.com/onasunnymorning/domain-os/issues/14) from onasunnymorning/APOS-226_PostmanCI
- Merge pull request [#13](https://github.com/onasunnymorning/domain-os/issues/13) from onasunnymorning/APOS-205-contract
- Merge pull request [#11](https://github.com/onasunnymorning/domain-os/issues/11) from onasunnymorning/APOS-222-singlePIbug
- Merge pull request [#12](https://github.com/onasunnymorning/domain-os/issues/12) from onasunnymorning/APOS-219-DOMREPO
- Merge pull request [#10](https://github.com/onasunnymorning/domain-os/issues/10) from onasunnymorning/APOS-221-DOCS
- Merge pull request [#9](https://github.com/onasunnymorning/domain-os/issues/9) from onasunnymorning/APOS-204-registrar
- Merge pull request [#8](https://github.com/onasunnymorning/domain-os/issues/8) from onasunnymorning/APOS-213-REPOTESTS
- Merge pull request [#7](https://github.com/onasunnymorning/domain-os/issues/7) from onasunnymorning/APOS-202-DOMAINS
- Merge pull request [#6](https://github.com/onasunnymorning/domain-os/issues/6) from onasunnymorning/APOS-192-HOSTS
- Merge pull request [#5](https://github.com/onasunnymorning/domain-os/issues/5) from onasunnymorning/APOS-193-Contact-Entities
- Merge pull request [#4](https://github.com/onasunnymorning/domain-os/issues/4) from onasunnymorning/APOS-202-Regsitrar-repo
- Merge pull request [#3](https://github.com/onasunnymorning/domain-os/issues/3) from onasunnymorning/APOS-194_RegistrarEntities
- Merge pull request [#2](https://github.com/onasunnymorning/domain-os/issues/2) from onasunnymorning/APOS-202_404insteadof500
- Merge pull request [#1](https://github.com/onasunnymorning/domain-os/issues/1) from onasunnymorning/APOS-175-NNDN


[Unreleased]: https://github.com/onasunnymorning/domain-os/compare/v0.6.1...HEAD
