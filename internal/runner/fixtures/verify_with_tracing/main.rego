package main

resource_types = {"null_resource"}

# all resources
resources[resource_type] = all {
	some resource_type
	resource_types[resource_type]
	all := [name |
		name := input.resource_changes[_]
		name.type == resource_type
	]
}

# number of creations of resources of a given type
num_creates[resource_type] = num {
	some resource_type
	resource_types[resource_type]
	all := resources[resource_type]
	creates := [res | res := all[_]; res.change.actions[_] == "create"]
	num := count(creates)
}

deny[msg] {
	num_resources := num_creates.null_resource
	trace("Print statement")
	num_resources > 0

	msg := "null resources cannot be created"
}

test_deny_null_created {
	count(deny[msg]) == 0 with input as {"resource_changes": [{"type": "null_resource", "change": {"actions": ["create"]}}]}
}
