# mini-auth-proxy

This reverse proxy can be used rewrite to a target while also adding a bearer token based on a config file. If you change the config file the server automatically reloads it and restarts itself. 

to start

      # create config.yaml
      go run github.com/gitu/mini-auth-proxy # starts the server
      # maybe also add a new entry to your hosts file to reroute based on a host name
