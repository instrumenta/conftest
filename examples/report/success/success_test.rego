package main

no_violations {
	count(deny) == 0
}

test_no_missing_label {
	input := {
		"kind": "Deployment",
		"metadata": {
			"name": "sample",
			"labels": {
				"app.kubernetes.io/name",
				"app.kubernetes.io/instance"
			}
		}
	}

	no_violations with input as input
}