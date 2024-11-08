package workflows

// QueueExpiringDomains is a function that queues expiring domains. It gets them from
// * get the domains from the api
// * queue them in the temporal expiring domain queue
