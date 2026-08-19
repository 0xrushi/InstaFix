package handlers

import "testing"

func TestNormalizeMediaType(t *testing.T) {
	tests := []struct {
		name     string
		media    Media
		isVideo  bool
		wantType string
	}{
		{
			name:     "Instagram image URL",
			media:    Media{URL: "https://scontent.cdninstagram.com/media/slide.jpg?token=example"},
			wantType: "GraphImage",
		},
		{
			name:     "video field without file extension",
			media:    Media{URL: "https://scontent.cdninstagram.com/media/playback"},
			isVideo:  true,
			wantType: "GraphVideo",
		},
		{
			name:     "preserves Instagram type",
			media:    Media{TypeName: "XDTGraphImage", URL: "https://example.com/media"},
			wantType: "XDTGraphImage",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			normalizeMediaType(&test.media, test.isVideo)
			if test.media.TypeName != test.wantType {
				t.Errorf("TypeName = %q, want %q", test.media.TypeName, test.wantType)
			}
		})
	}
}
