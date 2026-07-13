package henvflags

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"regexp"
	"strings"

	"github.com/open-feature/go-sdk/openfeature"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type InMemFeatureFlagProvider struct {
	OptionalLogger  *zap.Logger
	overridesByFlag map[string][]*OverrideVariant
}

type OverrideVariant struct {
	Flag          string            `json:"flag"`
	MatchCriteria map[string]string `json:"criteria"`
	OverrideValue interface{}       `json:"value"`

	matchPatterns map[string]*regexp.Regexp
}

// New constructs a new InMemFeatureFlagProvider with some initial overrides. These overrides are assumed to be valid
// and will panic if they are not. If the overrides may not be valid, use AddOverrides instead.
func New(initialOverrides ...*OverrideVariant) *InMemFeatureFlagProvider {
	p := new(InMemFeatureFlagProvider)
	if err := p.AddOverrides(initialOverrides); err != nil {
		panic(err)
	}
	return p
}

func (o *OverrideVariant) Prepare() error {
	if o.MatchCriteria == nil {
		o.MatchCriteria = map[string]string{}
	}
	o.matchPatterns = make(map[string]*regexp.Regexp, len(o.MatchCriteria))
	for k, v := range o.MatchCriteria {
		if strings.HasPrefix(v, "re:") {
			p, err := regexp.Compile(`^` + v[3:] + `$`)
			if err != nil {
				return errors.Errorf("bad regex flag '%s': %v", v, err)
			}
			o.matchPatterns[k] = p
		} else {
			o.matchPatterns[k] = regexp.MustCompile(`^` + regexp.QuoteMeta(v) + `$`)
		}
	}
	return nil
}

func (i *InMemFeatureFlagProvider) logger() *zap.Logger {
	if i.OptionalLogger != nil {
		return i.OptionalLogger
	}
	return zap.L()
}

func (i *InMemFeatureFlagProvider) AddJsonOverrides(raw string) error {
	var temp []*OverrideVariant
	if err := json.Unmarshal([]byte(raw), &temp); err != nil {
		return err
	}
	return i.AddOverrides(temp)
}

func (i *InMemFeatureFlagProvider) AddOverrides(variants []*OverrideVariant) error {
	if i.overridesByFlag == nil {
		i.overridesByFlag = make(map[string][]*OverrideVariant)
	}
	for index, variant := range variants {
		if variant.OverrideValue == nil || variant.Flag == "" {
			return fmt.Errorf("invalid override %d", index)
		}
		// copy the variant so that external changes do not interrupt it
		variant = &OverrideVariant{
			Flag:          variant.Flag,
			MatchCriteria: maps.Clone(variant.MatchCriteria),
			OverrideValue: variant.OverrideValue,
		}
		if err := variant.Prepare(); err != nil {
			return fmt.Errorf("invalid variant: %v", err)
		}
		current := i.overridesByFlag[variant.Flag]
		if current == nil {
			i.overridesByFlag[variant.Flag] = []*OverrideVariant{variant}
		} else {
			i.overridesByFlag[variant.Flag] = append(i.overridesByFlag[variant.Flag], variant)
		}
	}
	i.logger().Sugar().Infof("Added %d feature flag overrides", len(variants))
	return nil
}

func resolve(i *InMemFeatureFlagProvider, flag string, evalCtx openfeature.FlattenedContext, defaultValue any) openfeature.InterfaceResolutionDetail {
	if i.overridesByFlag != nil {
		if overrides, ok := i.overridesByFlag[flag]; ok {
		Outer:
			for _, override := range overrides {
				for k, expected := range override.matchPatterns {
					if actual := evalCtx[k]; actual != nil {
						actualString, stringOk := actual.(string)
						if !stringOk || !expected.MatchString(actualString) {
							continue Outer
						}
					} else {
						continue Outer
					}
				}
				return openfeature.InterfaceResolutionDetail{
					Value: override.OverrideValue,
					ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
						Reason: openfeature.TargetingMatchReason,
					},
				}

			}
		}
	}
	return openfeature.InterfaceResolutionDetail{
		Value: defaultValue,
		ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
			Reason: openfeature.DefaultReason,
		},
	}
}

func (i *InMemFeatureFlagProvider) Metadata() openfeature.Metadata {
	return openfeature.Metadata{Name: "in-mem-ff-provider"}
}

func (i *InMemFeatureFlagProvider) BooleanEvaluation(ctx context.Context, flag string, defaultValue bool, evalCtx openfeature.FlattenedContext) openfeature.BoolResolutionDetail {
	res := resolve(i, flag, evalCtx, defaultValue)
	v, ok := res.Value.(bool)
	if !ok {
		return openfeature.BoolResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewTypeMismatchResolutionError(""),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	return openfeature.BoolResolutionDetail{Value: v, ProviderResolutionDetail: res.ProviderResolutionDetail}
}

func (i *InMemFeatureFlagProvider) StringEvaluation(ctx context.Context, flag string, defaultValue string, evalCtx openfeature.FlattenedContext) openfeature.StringResolutionDetail {
	res := resolve(i, flag, evalCtx, defaultValue)
	v, ok := res.Value.(string)
	if !ok {
		return openfeature.StringResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewTypeMismatchResolutionError(""),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	return openfeature.StringResolutionDetail{Value: v, ProviderResolutionDetail: res.ProviderResolutionDetail}
}

func (i *InMemFeatureFlagProvider) FloatEvaluation(ctx context.Context, flag string, defaultValue float64, evalCtx openfeature.FlattenedContext) openfeature.FloatResolutionDetail {
	res := resolve(i, flag, evalCtx, defaultValue)
	v, ok := res.Value.(float64)
	if !ok {
		return openfeature.FloatResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewTypeMismatchResolutionError(""),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	return openfeature.FloatResolutionDetail{Value: v, ProviderResolutionDetail: res.ProviderResolutionDetail}
}

func (i *InMemFeatureFlagProvider) IntEvaluation(ctx context.Context, flag string, defaultValue int64, evalCtx openfeature.FlattenedContext) openfeature.IntResolutionDetail {
	res := resolve(i, flag, evalCtx, float64(defaultValue))
	v, ok := res.Value.(float64)
	if !ok {
		return openfeature.IntResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewTypeMismatchResolutionError(""),
				Reason:          openfeature.ErrorReason,
			},
		}
	}

	return openfeature.IntResolutionDetail{Value: int64(v), ProviderResolutionDetail: res.ProviderResolutionDetail}
}

func (i *InMemFeatureFlagProvider) ObjectEvaluation(ctx context.Context, flag string, defaultValue interface{}, evalCtx openfeature.FlattenedContext) openfeature.InterfaceResolutionDetail {
	res := resolve(i, flag, evalCtx, defaultValue)
	if res.Value == nil {
		return openfeature.InterfaceResolutionDetail{
			Value: defaultValue,
			ProviderResolutionDetail: openfeature.ProviderResolutionDetail{
				ResolutionError: openfeature.NewTypeMismatchResolutionError(""),
				Reason:          openfeature.ErrorReason,
			},
		}
	}
	return openfeature.InterfaceResolutionDetail{Value: res.Value, ProviderResolutionDetail: res.ProviderResolutionDetail}
}

func (i *InMemFeatureFlagProvider) Hooks() []openfeature.Hook {
	return []openfeature.Hook{}
}

var _ openfeature.FeatureProvider = (*InMemFeatureFlagProvider)(nil)
