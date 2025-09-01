package contentstuff

import (
	// testify
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFrontmatterParsing(t *testing.T) {
	testify := assert.New(t)
	yamlContent := `---
title: "Sample YAML"
---
`
	fm, _, err := ExtractFrontmatter([]byte(yamlContent))
	testify.NoError(err)
	testify.Len(fm.Data, 1)
	testify.Len(fm.DataKV, 1)

	newYAMLContent := `---
title_removed: "Sample YAML"
---
`
	err = fm.SetRaw([]byte(newYAMLContent))
	testify.NoError(err)
	testify.Len(fm.Data, 1)
	testify.Len(fm.DataKV, 1)
	testify.True(fm.HasKey("title_removed"))
	testify.False(fm.HasKey("title"))
}
