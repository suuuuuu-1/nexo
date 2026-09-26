package handler

import "testing"

func TestValidEpisodeUpload(t *testing.T) {
	tests := []struct {
		name string
		req  episodeUploadRequest
		want bool
	}{
		{
			name: "normal mp4",
			req:  episodeUploadRequest{Filename: "episode.mp4", ContentType: "video/mp4", SizeBytes: 1024},
			want: true,
		},
		{
			name: "case insensitive extension",
			req:  episodeUploadRequest{Filename: "episode.MP4", ContentType: "video/mp4", SizeBytes: 1024},
			want: true,
		},
		{
			name: "reject non mp4",
			req:  episodeUploadRequest{Filename: "episode.mov", ContentType: "video/quicktime", SizeBytes: 1024},
			want: false,
		},
		{
			name: "reject wrong content type",
			req:  episodeUploadRequest{Filename: "episode.mp4", ContentType: "application/octet-stream", SizeBytes: 1024},
			want: false,
		},
		{
			name: "reject empty file",
			req:  episodeUploadRequest{Filename: "episode.mp4", ContentType: "video/mp4", SizeBytes: 0},
			want: false,
		},
		{
			name: "reject over limit",
			req:  episodeUploadRequest{Filename: "episode.mp4", ContentType: "video/mp4", SizeBytes: maxEpisodeUploadBytes + 1},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validEpisodeUpload(tt.req); got != tt.want {
				t.Errorf("validEpisodeUpload() = %t, want %t", got, tt.want)
			}
		})
	}
}
