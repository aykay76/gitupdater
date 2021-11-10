# gitupdater

A microservice that subscribes to a Redis Stream, takes the payload and stores it in a GitHub repository.

My initial motivation for this was born out of `autograf` (see my repository with the same name) where I want to automate the creation of Grafana dashboards. Hopefully this microservice could be useful for any scenario where payloads get sent through event driven methods and you might want to store the result in a repo.

