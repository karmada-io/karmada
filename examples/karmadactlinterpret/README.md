# Examples for Karmadactl interpret command

This example shows how to use the `karmadactl interpret` command easily.

*Validate the ResourceInterpreterCustomization configuration*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --check
```

*Execute the InterpretReplica rule*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --observed-file observed-deploy-nginx.yaml --operation=InterpretReplica
```

*Execute the Retain rule*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --desired-file desired-deploy-nginx.yaml --observed-file observed-deploy-nginx.yaml --operation Retain
```

*Execute the InterpretStatus rule*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --observed-file observed-deploy-nginx.yaml --operation InterpretStatus
```

*Execute the InterpretHealth rule*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --observed-file observed-deploy-nginx.yaml --operation InterpretHealth
```

*Execute the InterpretDependency rule*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --desired-file desired-deploy-nginx.yaml --operation InterpretDependency
```

*Execute the AggregateStatus rule*

```shell
karmadactl interpret -f resourceinterpretercustomization.yaml --desired-file desired-deploy-nginx.yaml --operation AggregateStatus --status-file status-file.yaml
```
