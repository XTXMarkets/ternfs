// Copyright 2026 XTX Markets Technologies Limited
//
// SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

package main

// payloadBytesExpr returns a Go expression that converts a payload read result
// variable to []byte for storage in a union struct. The expression depends on
// whether the payload is a struct (r.m[:]), a struct-reader with data (r.data),
// or a []byte alias ([]byte(r)).
func (g *Generator) payloadBytesExpr(payloadTi *TypeInfo) string {
	if payloadTi.Decl.Kind == DeclStruct {
		return "r.m[:]"
	}
	if payloadTi.IsMultiArray || len(payloadTi.VarSectionBoundaries) > 0 {
		return "r.data"
	}
	return "[]byte(r)"
}

func (g *Generator) emitUnion(ti *TypeInfo) {
	u := ti.Decl.Union
	name := g.goName(ti)

	g.p("\n" + sectionSep)
	g.p("// %s — union on %s (external discriminant)\n", name, u.DiscType)
	g.p(sectionSep)

	// Type declaration — struct with byte slice and discriminant for safety checks.
	discEnum := g.a.Types[u.DiscType]
	discGoType := enumGoType(discEnum.Decl.Enum)
	g.p("\ntype %s struct {\n\tb []byte\n\tdisc %s\n}\n", name, discGoType)

	discParam := camelCase(u.DiscType)

	// Cursor-advancing read.
	g.p("\nfunc read%s(b *[]byte, %s %s) (%s, bool) {\n",
		name, discParam, discGoType, name)
	g.p("\tswitch %s {\n", discParam)

	emitArmRead := func(arm UnionArm) {
		if arm.Payload == "void" {
			g.p("\t\treturn %s{b: (*b)[:0], disc: %s}, true\n", name, discParam)
		} else {
			payloadTi := g.a.Types[arm.Payload]
			g.p("\t\tr, ok := read%s(b)\n", g.goName(payloadTi))
			g.p("\t\tif !ok {\n\t\t\treturn %s{}, false\n\t\t}\n", name)
			g.p("\t\treturn %s{b: %s, disc: %s}, true\n", name, g.payloadBytesExpr(payloadTi), discParam)
		}
	}

	var defaultArm *UnionArm
	for _, arm := range u.Arms {
		if arm.Label == "default" {
			defaultArm = &arm
			continue
		}
		g.p("\tcase %s:\n", arm.Label)
		emitArmRead(arm)
	}

	g.p("\tdefault:\n")
	if defaultArm != nil {
		emitArmRead(*defaultArm)
	} else {
		g.p("\t\treturn %s{}, false\n", name)
	}
	g.p("\t}\n}\n")

	// Detect payload types used by multiple arms (need per-arm naming).
	payloadCount := map[string]int{}
	for _, arm := range u.Arms {
		if arm.Payload != "void" && arm.Label != "default" {
			payloadCount[arm.Payload]++
		}
	}

	// As-accessors: one per arm, each checks exactly one disc value.
	for _, arm := range u.Arms {
		if arm.Payload == "void" || arm.Label == "default" {
			continue
		}
		payloadTi := g.a.Types[arm.Payload]

		// Name: use payload type if unique, arm label if shared.
		methodName := g.goName(payloadTi)
		if payloadCount[arm.Payload] > 1 {
			methodName = armSuffix(stripEnumPrefix(arm.Label, discEnum.Decl.Enum.Constants))
		}

		g.p("\nfunc (m %s) As%s() %s {\n", name, methodName, g.goName(payloadTi))
		g.p("\tif m.disc != %s {\n\t\tpanic(\"wrong union discriminant\")\n\t}\n", arm.Label)
		if payloadTi.Decl.Kind == DeclStruct {
			sizeConst := sizeConstName(arm.Payload)
			g.p("\treturn %s{m: (*[%s]byte)(m.b)}\n}\n", g.goName(payloadTi), sizeConst)
		} else if payloadTi.IsExtentStruct {
			g.p("\treturn %s{m: (*[%d]byte)(m.b)}\n}\n", g.goName(payloadTi), payloadTi.ExtentTotalSize)
		} else if g.a.isStructReader(arm.Payload) {
			g.p("\tv, _ := Read%s(m.b)\n\treturn v\n}\n", g.goName(payloadTi))
		} else {
			g.p("\treturn %s(m.b)\n}\n", g.goName(payloadTi))
		}
	}
	// Default arm with non-void payload.
	emittedDefault := false
	for _, arm := range u.Arms {
		if arm.Label == "default" && arm.Payload != "void" && !emittedDefault {
			emittedDefault = true
			payloadTi := g.a.Types[arm.Payload]
			g.p("\nfunc (m %s) As%s() %s {\n", name, g.goName(payloadTi), g.goName(payloadTi))
			if g.a.isStructReader(arm.Payload) {
				g.p("\tv, _ := Read%s(m.b)\n\treturn v\n}\n", g.goName(payloadTi))
			} else {
				g.p("\treturn %s(m.b)\n}\n", g.goName(payloadTi))
			}
		}
	}
}
