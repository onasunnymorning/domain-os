# Point Tilt at the existing docker-compose configuration.
docker_compose("./docker-compose.yml")

# Clean up dangling images and containers
docker_prune_settings(disable = False , max_age_mins = 360 , num_builds = 0 , interval_hrs = 1 , keep_recent = 2) 

trigger_mode(TRIGGER_MODE_MANUAL)

# Group resources by labels
dc_resource('admin-frontend', labels=["frontend"])
dc_resource('db', labels=["backend"])
dc_resource('admin-api', labels=["backend"])
dc_resource('msg-broker', labels=["events"])
dc_resource('msg-client', labels=["events"])