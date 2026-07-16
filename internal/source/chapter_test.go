package source

import "testing"

func TestChapterDisplayTitle(t *testing.T) {
	tests := []struct {
		name          string
		chapter       *Chapter
		fallbackIndex int
		want          string
	}{
		{
			name:          "nil chapter uses fallback index",
			chapter:       nil,
			fallbackIndex: 2,
			want:          "Unknown chapter #3",
		},
		{
			name:          "index and title",
			chapter:       &Chapter{Index: " 1 ", Title: " Intro "},
			fallbackIndex: 0,
			want:          "Chapter 1 - Intro",
		},
		{
			name:          "index only",
			chapter:       &Chapter{Index: " 2 "},
			fallbackIndex: 0,
			want:          "Chapter 2",
		},
		{
			name:          "title only",
			chapter:       &Chapter{Title: " Special "},
			fallbackIndex: 0,
			want:          "Special",
		},
		{
			name:          "empty chapter uses fallback index",
			chapter:       &Chapter{},
			fallbackIndex: 4,
			want:          "Unknown chapter #5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chapter.DisplayTitle(tt.fallbackIndex); got != tt.want {
				t.Fatalf("DisplayTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChapterDisplayName(t *testing.T) {
	tests := []struct {
		name    string
		chapter *Chapter
		want    string
	}{
		{name: "nil chapter", chapter: nil, want: "Unknown chapter"},
		{name: "index and title", chapter: &Chapter{Index: " 1 ", Title: " Intro "}, want: "Chapter 1 - Intro"},
		{name: "index only", chapter: &Chapter{Index: " 2 "}, want: "Chapter 2"},
		{name: "title only", chapter: &Chapter{Title: " Special "}, want: "Special"},
		{name: "empty chapter", chapter: &Chapter{}, want: "Unknown chapter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chapter.DisplayName(); got != tt.want {
				t.Fatalf("DisplayName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestChapterLogName(t *testing.T) {
	tests := []struct {
		name    string
		chapter *Chapter
		want    string
	}{
		{name: "nil chapter", chapter: nil, want: "unknown chapter"},
		{name: "index and title", chapter: &Chapter{Index: " 1 ", Title: " Intro "}, want: "chapter 1 (Intro)"},
		{name: "index only", chapter: &Chapter{Index: " 2 "}, want: "chapter 2"},
		{name: "title only", chapter: &Chapter{Title: " Special "}, want: "Special"},
		{name: "empty chapter", chapter: &Chapter{}, want: "unknown chapter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.chapter.LogName(); got != tt.want {
				t.Fatalf("LogName() = %q, want %q", got, tt.want)
			}
		})
	}
}
