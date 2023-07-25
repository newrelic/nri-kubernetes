package definition

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromRawFetchesProperly(t *testing.T) {
	raw := RawGroups{
		"group1": {
			"entity1": {
				"metric_name_1": "metric_value_1",
				"metric_name_2": "metric_value_2",
			},
			"entity2": {
				"metric_name_3": "metric_value_3",
				"metric_name_4": "metric_value_4",
				"metric_name_5": "metric_value_5",
			},
		},
	}

	v, err := FromRaw("metric_name_3")("group1", "entity2", raw)
	assert.NoError(t, err)
	assert.Equal(t, "metric_value_3", v)
}

func TestFromRawErrorsOnNotFound(t *testing.T) {
	raw := RawGroups{
		"group1": {
			"entity1": {
				"metric_name_1": "metric_value_1",
				"metric_name_2": "metric_value_2",
			},
			"entity2": {
				"metric_name_3": "metric_value_3",
				"metric_name_4": "metric_value_4",
				"metric_name_5": "metric_value_5",
			},
		},
	}

	v, err := FromRaw("metric_name_3")("nonExistingGroup", "entity2", raw)
	assert.EqualError(t, err, "group \"nonExistingGroup\" not found")
	assert.Nil(t, v)

	v, err = FromRaw("metric_name_3")("group1", "nonExistingEntity", raw)
	assert.EqualError(t, err, "entity \"nonExistingEntity\" not found")
	assert.Nil(t, v)

	v, err = FromRaw("non_existing_metric")("group1", "entity2", raw)
	assert.EqualError(t, err, "metric \"non_existing_metric\" not found")
	assert.Nil(t, v)
}

func TestTransform(t *testing.T) {
	raw := RawGroups{
		"group1": {
			"entity1": {
				"metric_name_1": "metric_value_1",
				"metric_name_2": "metric_value_2",
			},
			"entity2": {
				"metric_name_3": "metric_value_3",
				"metric_name_4": "metric_value_4",
				"metric_name_5": "metric_value_5",
			},
		},
	}

	v, err := Transform(FromRaw("metric_name_3"),
		func(in FetchedValue) (FetchedValue, error) {
			return strings.ToUpper(in.(string)), nil
		})("group1", "entity2", raw)
	assert.NoError(t, err)
	assert.Equal(t, "METRIC_VALUE_3", v)
}

func TestTransformBypassesError(t *testing.T) {
	raw := RawGroups{
		"group1": {
			"entity1": {
				"metric_name_1": "metric_value_1",
				"metric_name_2": "metric_value_2",
			},
			"entity2": {
				"metric_name_3": "metric_value_3",
				"metric_name_4": "metric_value_4",
				"metric_name_5": "metric_value_5",
			},
		},
	}

	transformFunc := func(in FetchedValue) (FetchedValue, error) {
		return in, nil
	}

	v, err := Transform(FromRaw("metric_name_3"), transformFunc)("nonExistingGroup", "entity2", raw)
	assert.EqualError(t, err, "group \"nonExistingGroup\" not found")
	assert.Nil(t, v)

	v, err = Transform(FromRaw("metric_name_3"), transformFunc)("group1", "nonExistingEntity", raw)
	assert.EqualError(t, err, "entity \"nonExistingEntity\" not found")
	assert.Nil(t, v)

	v, err = Transform(FromRaw("non_existing_metric"), transformFunc)("group1", "entity2", raw)
	assert.EqualError(t, err, "metric \"non_existing_metric\" not found")
	assert.Nil(t, v)
}

var (
	errDummyFilter    = fmt.Errorf("dummy filter error")
	errDummyTransform = fmt.Errorf("dummy transform error")
)

func TestTransformAndFilter(t *testing.T) { //nolint: funlen
	t.Parallel()
	type args struct {
		fetchFunc     FetchFunc
		filterFunc    FilterFunc
		transformFunc TransformFunc
		groupLabel    string
		entityID      string
		raw           RawGroups
	}
	tests := []struct {
		name    string
		args    args
		want    FetchedValue
		wantErr string
	}{
		{
			name: "FetchFuncError",
			args: args{
				fetchFunc: FromRaw("dummy_metric"),
				filterFunc: func(value FetchedValue, groupLabel, entityID string, groups RawGroups) (FilteredValue, error) {
					return 241414124, nil
				},
				transformFunc: func(value FetchedValue) (FetchedValue, error) {
					return 24124124, nil
				},
				groupLabel: "group1",
				entityID:   "entity1",
				raw: RawGroups{
					"group1": {
						"entity1": {
							"metric_name_1": "metric_value_1",
						},
					},
				},
			},
			want:    nil,
			wantErr: "metric \"dummy_metric\" not found",
		},
		{
			name: "FilterFuncError",
			args: args{
				fetchFunc: FromRaw("metric_name_1"),
				filterFunc: func(value FetchedValue, groupLabel, entityID string, groups RawGroups) (FilteredValue, error) {
					return nil, errDummyFilter
				},
				transformFunc: func(value FetchedValue) (FetchedValue, error) {
					return 24124124, nil
				},
				groupLabel: "group1",
				entityID:   "entity1",
				raw: RawGroups{
					"group1": {
						"entity1": {
							"metric_name_1": "metric_value_1",
						},
					},
				},
			},
			want:    nil,
			wantErr: "dummy filter error",
		},
		{
			name: "TransformFuncError",
			args: args{
				fetchFunc: FromRaw("metric_name_1"),
				filterFunc: func(value FetchedValue, groupLabel, entityID string, groups RawGroups) (FilteredValue, error) {
					return 24124124, nil
				},
				transformFunc: func(value FetchedValue) (FetchedValue, error) {
					return nil, errDummyTransform
				},
				groupLabel: "group1",
				entityID:   "entity1",
				raw: RawGroups{
					"group1": {
						"entity1": {
							"metric_name_1": "metric_value_1",
						},
					},
				},
			},
			want:    nil,
			wantErr: "dummy transform error",
		},
		{
			name: "NoError",
			args: args{
				fetchFunc: FromRaw("metric_name_1"),
				filterFunc: func(value FetchedValue, groupLabel, entityID string, groups RawGroups) (FilteredValue, error) {
					return 24124124, nil
				},
				transformFunc: func(value FetchedValue) (FetchedValue, error) {
					return 5686865, nil
				},
				groupLabel: "group1",
				entityID:   "entity1",
				raw: RawGroups{
					"group1": {
						"entity1": {
							"metric_name_1": "metric_value_1",
						},
					},
				},
			},
			want:    24124124,
			wantErr: "",
		},
	}
	for _, testCase := range tests {
		tt := testCase
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			value, err := TransformAndFilter(tt.args.fetchFunc, tt.args.transformFunc, tt.args.filterFunc)(tt.args.groupLabel, tt.args.entityID, tt.args.raw)
			if len(tt.wantErr) > 0 {
				assert.EqualError(t, err, tt.wantErr, "wanted error %s, got %s", tt.wantErr, err.Error())
			} else {
				assert.NoError(t, err)
			}
			assert.EqualValuesf(t, tt.want, value, "wanted val %v, got %v", tt.want, value)
		})
	}
}
