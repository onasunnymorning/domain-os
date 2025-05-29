# domain-os

Domain (Registry for now) Operating System (DOS)

 # Why?
 
 The registry backend business operates in a high-volume low-margin setup, therefore a highly automated, robust, adaptable system that is cost efficient to operate is required. 
 The current legacy systems are often monolithic, hard to maintain, and difficult to adapt to new requirements.
 They are often built on outdated technology stacks that are not well suited for the current and future needs of the industry.
 It should be flexible in terms of policy, infrastructure and quick to evolve and integrate. 

 # How?

By taking the accumulated knowledge of working in the industry and building a system that is modular, flexible, and easy to operate.


 Offer clear context, a blazing fast developer experience and feedback loop and lots of automated testing for safety.
 I've tried to focus on making this system easy to operate with a low overhead. While it is not yet optimized, it has testing, automation, visibility, and an open architecture and documentation.
 This way, when optimization is needed, it will be staightforward as we will have data and visibility to evolve the system quickly in the desired direction.
 The data model aligns with the RFCs and can easily be evolved, optimized or even be implemented with a different database because its de-coupled from the business logic.

  # What?
  A registry system with core functionality that you can easily integrate into your existing service stack or build on top of

# Running the app

## Requirements

* You do need docker desktop installed 
* An API client, I’m using Postman in this video

## Running the app

### No Code

Video walkthrough: https://youtu.be/pobt7sm7ixw

* Download this zip file containing a docker-compose file and a basic .env file: https://drive.google.com/file/d/1dbsusOJ2g1FPLJ0rUBkYc3Av8PABgNab/view?usp=sharing
* Unzip the file and open a terminal in the folder
* Run `BRANCH=latest docker compose --profile essential up`

* Open http://localhost:8080/swagger/index.html and download the Postman collection (doc.json)
* Import the Postman collection into your Postman client
* Configure the environment variables in Postman (baseUrl and token) to match the .env file 
* Send your first request to the API

If you want to populate the system, start by creating a Registry Operator, then a TLD, Setup a Phase to enable the TLD, create Registrars, and then create Domains...



