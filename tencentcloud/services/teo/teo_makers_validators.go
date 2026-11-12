package teo

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
)

// makersStringCheck adapts a plain check function to validator.String. The
// check returns an empty string when the value is valid.
type makersStringCheck struct {
	description string
	check       func(string) string
}

func (v makersStringCheck) Description(_ context.Context) string         { return v.description }
func (v makersStringCheck) MarkdownDescription(_ context.Context) string { return v.description }

func (v makersStringCheck) ValidateString(_ context.Context, req validator.StringRequest, resp *validator.StringResponse) {
	if req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	if msg := v.check(req.ConfigValue.ValueString()); msg != "" {
		resp.Diagnostics.AddAttributeError(req.Path, "Invalid attribute value", msg)
	}
}

func makersOneOf(values ...string) validator.String {
	return makersStringCheck{
		description: "value must be one of: " + strings.Join(values, ", "),
		check: func(s string) string {
			for _, v := range values {
				if s == v {
					return ""
				}
			}
			return fmt.Sprintf("%q must be one of: %s", s, strings.Join(values, ", "))
		},
	}
}

func makersMatches(re *regexp.Regexp) validator.String {
	return makersStringCheck{
		description: "value must match " + re.String(),
		check: func(s string) string {
			if re.MatchString(s) {
				return ""
			}
			return fmt.Sprintf("%q must match %s", s, re.String())
		},
	}
}

// makersTrimmed rejects surrounding whitespace: the platform trims it on write,
// which would otherwise show up as a permanent diff.
func makersTrimmed(maxLen int) validator.String {
	return makersStringCheck{
		description: fmt.Sprintf("value must not have leading or trailing whitespace and must be at most %d characters", maxLen),
		check: func(s string) string {
			if strings.TrimSpace(s) != s {
				return "value must not have leading or trailing whitespace"
			}
			if len(s) > maxLen {
				return fmt.Sprintf("value must be at most %d characters", maxLen)
			}
			return ""
		},
	}
}
