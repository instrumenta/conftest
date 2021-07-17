package main

deny_root[result] {
	input.kind == "Deployment"
	c = input.spec.template.spec.containers[_]
	not c.securityContext.runAsNonRoot

	# key "msg" is required to be set.
	result := {
		"container": c.name,
		"deployment": input.metadata.name,
		"msg": sprintf("container %s in deployment %s doesn't set runAsNonRoot", [c.name, input.metadata.name]),
	}
}

root_exceptions = [{"deployment": "mydep", "containers": ["host-agent"]}]

# Here the exception I want to be able to express is "mydep can run host-agent as root".
# But not web as root
exclude_root[attrs] {
	deployment := input.metadata.name
	container := input.spec.template.spec.containers[_].name
	exception := root_exceptions[_]

	deployment == exception.deployment
	container == exception.containers[_]

	attrs = [{"container": container, "deployment": deployment}]
}
