deps:
	sudo chmod +x sloth


karmada-upstream-slo.yaml: karmada-upstream-config.yaml
	./sloth generate -i $< -o $@
generate: karmada-upstream-slo.yaml


