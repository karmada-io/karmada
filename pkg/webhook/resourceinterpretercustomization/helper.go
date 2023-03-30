package resourceinterpretercustomization

import (
	"context"
	"fmt"
	"time"

	configv1alpha1 "github.com/karmada-io/karmada/pkg/apis/config/v1alpha1"
	"github.com/karmada-io/karmada/pkg/resourceinterpreter/customized/declarative/luavm"
	"github.com/karmada-io/karmada/pkg/util/interpreter"
)

func validateCustomizationRule(oldRules, newRules *configv1alpha1.ResourceInterpreterCustomization) error {
	if oldRules.Spec.Target.APIVersion != newRules.Spec.Target.APIVersion ||
		oldRules.Spec.Target.Kind != newRules.Spec.Target.Kind {
		return nil
	}
	for _, rule := range interpreter.AllResourceInterpreterCustomizationRules {
		// skip InterpretDependency operation because it supports multiple rules.
		if rule.Name() == string(configv1alpha1.InterpreterOperationInterpretDependency) {
			continue
		}
		oldScript := rule.GetScript(oldRules)
		newScript := rule.GetScript(newRules)
		if oldScript != "" && newScript != "" {
			return fmt.Errorf("conflicting with InterpreterOperation(%s) of existing ResourceInterpreterCustomization(%s)", rule.Name(), oldRules.Name)
		}
	}
	return nil
}

func checkCustomizationsRule(customization *configv1alpha1.ResourceInterpreterCustomization) error {
	ctx, cancel := context.WithTimeout(context.TODO(), time.Second)
	defer cancel()
	l, err := luavm.NewWithContext(ctx)
	if err != nil {
		return err
	}
	defer l.Close()
	for _, rule := range interpreter.AllResourceInterpreterCustomizationRules {
		if script := rule.GetScript(customization); script != "" {
			if _, err = l.LoadString(script); err != nil {
				return fmt.Errorf("InterpreterOperation(%s) Lua script error: %v", rule.Name(), err)
			}
		}
	}
	return nil
}

func validateResourceInterpreterCustomizations(newConfig *configv1alpha1.ResourceInterpreterCustomization, customizations *configv1alpha1.ResourceInterpreterCustomizationList) error {
	for _, config := range customizations.Items {
		// skip self verification
		if config.Name == newConfig.Name {
			continue
		}
		oldConfig := config
		if err := validateCustomizationRule(&oldConfig, newConfig); err != nil {
			return err
		}
	}
	return checkCustomizationsRule(newConfig)
}
