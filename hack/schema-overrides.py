#!/usr/bin/env python3
"""Tighten the values.schema.json written by readme-generator-for-helm.

The generator infers every type from the default value, so a field that defaults to "" but also
takes a number (ports, grace periods, PDB counts) would reject the number. This script loosens
those fields and adds enums for fields with a fixed set of values.
Usage: hack/schema-overrides.py charts/<chart>/values.schema.json charts/<chart>/schema-overrides.json
"""
import json
import sys

schema_path, overrides_path = sys.argv[1], sys.argv[2]
with open(schema_path) as f:
    schema = json.load(f)
with open(overrides_path) as f:
    overrides = json.load(f)

for dotted, patch in overrides.items():
    node = schema
    for part in dotted.split('.'):
        node = node['properties'][part]
    node.update(patch)

with open(schema_path, 'w') as f:
    json.dump(schema, f, indent=4)
    f.write('\n')
