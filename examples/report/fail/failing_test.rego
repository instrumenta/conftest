package main

no_violations {
	count(deny) == 0
}

test_missing_required_label_fail {
	input := {
		"kind": "Deployment",
		"metadata": {
			"name": "sample",
			"labels": {
				"app.kubernetes.io/instance"
			}
		}
	}

	no_violations with input as input
}