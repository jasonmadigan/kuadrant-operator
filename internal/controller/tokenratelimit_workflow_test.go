//go:build unit

package controllers

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"
	k8stypes "k8s.io/apimachinery/pkg/types"

	kuadrantv1 "github.com/kuadrant/kuadrant-operator/api/v1"
	kuadrantv1alpha1 "github.com/kuadrant/kuadrant-operator/api/v1alpha1"
	"github.com/kuadrant/kuadrant-operator/internal/wasm"
)

func TestTokenLimitNameToLimitadorIdentifier(t *testing.T) {
	testCases := []struct {
		name            string
		tlrpKey         k8stypes.NamespacedName
		uniqueLimitName string
		expected        *regexp.Regexp
	}{
		{
			name:            "prepends the token limitador limit identifier prefix",
			tlrpKey:         k8stypes.NamespacedName{Namespace: "testNS", Name: "tlrpA"},
			uniqueLimitName: "foo",
			expected:        regexp.MustCompile(`^tokenlimit\.foo.+`),
		},
		{
			name:            "sanitizes invalid chars for token limits",
			tlrpKey:         k8stypes.NamespacedName{Namespace: "testNS", Name: "tlrpA"},
			uniqueLimitName: "my/token-limit-0",
			expected:        regexp.MustCompile(`^tokenlimit\.my_token_limit_0.+$`),
		},
		{
			name:            "sanitizes the dot char (.) for token limits",
			tlrpKey:         k8stypes.NamespacedName{Namespace: "testNS", Name: "tlrpA"},
			uniqueLimitName: "my.token.limit",
			expected:        regexp.MustCompile(`^tokenlimit\.my_token_limit.+$`),
		},
		{
			name:            "appends a hash for token limits to avoid breaking uniqueness",
			tlrpKey:         k8stypes.NamespacedName{Namespace: "testNS", Name: "tlrpA"},
			uniqueLimitName: "foo",
			expected:        regexp.MustCompile(`^.+__5b761c62$`),
		},
		{
			name:            "different tlrp keys result in different identifiers",
			tlrpKey:         k8stypes.NamespacedName{Namespace: "testNS", Name: "tlrpB"},
			uniqueLimitName: "foo",
			expected:        regexp.MustCompile(`^.+__5031687f$`),
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(subT *testing.T) {
			// Use the same function as RateLimitPolicy but with "tokenlimit" prefix
			identifier := TokenLimitNameToLimitadorIdentifier(tc.tlrpKey, tc.uniqueLimitName)
			if !tc.expected.MatchString(identifier) {
				subT.Errorf("identifier does not match, expected(%s), got (%s)", tc.expected, identifier)
			}
		})
	}
}

func TestWasmActionFromTokenLimit(t *testing.T) {
	testCases := []struct {
		name               string
		tokenLimit         *kuadrantv1alpha1.TokenLimit
		limitIdentifier    string
		scope              string
		topLevelPredicates kuadrantv1.WhenPredicates
		expectedAction     wasm.Action
	}{
		{
			name:            "token limit without conditions nor counters",
			tokenLimit:      &kuadrantv1alpha1.TokenLimit{},
			limitIdentifier: "tokenlimit.myTokenLimit__d681f6c3",
			scope:           "my-ns/my-route",
			expectedAction: wasm.Action{
				ServiceName: wasm.RateLimitServiceName,
				Scope:       "my-ns/my-route",
				Data: []wasm.DataType{
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "tokenlimit.myTokenLimit__d681f6c3",
								Value: "1",
							},
						},
					},
				},
			},
		},
		{
			name: "token limit with counter expressions",
			tokenLimit: &kuadrantv1alpha1.TokenLimit{
				Counters: []kuadrantv1alpha1.TokenCounter{{Expression: "request.headers['x-api-key']"}},
			},
			limitIdentifier: "tokenlimit.myTokenLimit__d681f6c3",
			scope:           "my-ns/my-route",
			expectedAction: wasm.Action{
				ServiceName: wasm.RateLimitServiceName,
				Scope:       "my-ns/my-route",
				Data: []wasm.DataType{
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "tokenlimit.myTokenLimit__d681f6c3",
								Value: "1",
							},
						},
					},
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "request.headers['x-api-key']",
								Value: "request.headers['x-api-key']",
							},
						},
					},
				},
			},
		},
		{
			name: "token limit with counter expressions and when predicates",
			tokenLimit: &kuadrantv1alpha1.TokenLimit{
				Counters: []kuadrantv1alpha1.TokenCounter{{Expression: "auth.identity.username"}},
				When:     kuadrantv1.NewWhenPredicates("has(auth.identity.username)"),
			},
			limitIdentifier: "tokenlimit.myTokenLimit__d681f6c3",
			scope:           "my-ns/my-route",
			expectedAction: wasm.Action{
				ServiceName: wasm.RateLimitServiceName,
				Scope:       "my-ns/my-route",
				Predicates:  []string{"has(auth.identity.username)"},
				Data: []wasm.DataType{
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "tokenlimit.myTokenLimit__d681f6c3",
								Value: "1",
							},
						},
					},
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "auth.identity.username",
								Value: "auth.identity.username",
							},
						},
					},
				},
			},
		},
		{
			name:               "token limit with top level predicates and no when predicates",
			tokenLimit:         &kuadrantv1alpha1.TokenLimit{},
			topLevelPredicates: kuadrantv1.NewWhenPredicates("request.method == 'POST'"),
			limitIdentifier:    "tokenlimit.myTokenLimit__d681f6c3",
			scope:              "my-ns/my-route",
			expectedAction: wasm.Action{
				ServiceName: wasm.RateLimitServiceName,
				Scope:       "my-ns/my-route",
				Predicates:  []string{"request.method == 'POST'"},
				Data: []wasm.DataType{
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "tokenlimit.myTokenLimit__d681f6c3",
								Value: "1",
							},
						},
					},
				},
			},
		},
		{
			name: "token limit with top level predicates and when predicates",
			tokenLimit: &kuadrantv1alpha1.TokenLimit{
				When: kuadrantv1.NewWhenPredicates("has(request.headers['x-api-key'])"),
			},
			topLevelPredicates: kuadrantv1.NewWhenPredicates("request.path.startsWith('/api/')"),
			limitIdentifier:    "tokenlimit.myTokenLimit__d681f6c3",
			scope:              "my-ns/my-route",
			expectedAction: wasm.Action{
				ServiceName: wasm.RateLimitServiceName,
				Scope:       "my-ns/my-route",
				Predicates: []string{
					"request.path.startsWith('/api/')",
					"has(request.headers['x-api-key'])",
				},
				Data: []wasm.DataType{
					{
						Value: &wasm.Expression{
							ExpressionItem: wasm.ExpressionItem{
								Key:   "tokenlimit.myTokenLimit__d681f6c3",
								Value: "1",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			computedRule := wasmActionFromTokenLimit(tc.tokenLimit, tc.limitIdentifier, tc.scope, tc.topLevelPredicates)
			if diff := cmp.Diff(tc.expectedAction, computedRule); diff != "" {
				t.Errorf("unexpected wasm rule (-want +got):\n%s", diff)
			}
		})
	}
}
