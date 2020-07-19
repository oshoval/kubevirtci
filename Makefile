cluster-up:
	./cluster-up/check.sh
	./cluster-up/up.sh

cluster-down:
	./cluster-up/down.sh

bump:
	./hack/bump.sh "$(provider)" "$(hash)"

.PHONY: \
	cluster-up \
	cluster-down \
	bump
