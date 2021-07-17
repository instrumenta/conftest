package main

exceptions = {"exception-name"}

deny_name[result] {
	input.resource[_][name]
	contains(name, "-")
	msg := sprintf("Resource Name '%s' contains dashes", [name])
	result := {
		"msg": msg,
		"resource-name": name,
	}
}

deny_resource_type[msg] {
	input.resource[type]
	type == "invalid_type"
	msg := sprintf("Resource Type '%s' is invalid", [type])
}

exclude_name[attrs] {
	exceptions[name]
	attrs := [{"resource-name": name}]
}

exception[rules] {
	rules := ["resource_type"]
}
