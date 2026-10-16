package domain

import (
	"os"
	"slices"
	"testing"
)

func TestPlainTextAndLinkURLs(t *testing.T) {
	post, err := os.ReadFile("testdata/serialize/post.golden")
	if err != nil {
		t.Fatal(err)
	}
	want := "Осенняя распродажа\nСкидки до 50% — только неделю. Подробности: https://shop.example.test/sale\n\nЦитата с выделением"
	if got := BodyPlainText(string(post)); got != want {
		t.Fatalf("BodyPlainText:\n got %q\nwant %q", got, want)
	}
	if got := BodyLinkURLs(string(post)); !slices.Equal(got, []string{
		"https://shop.example.test/sale?utm_source=max&utm_medium=channel_ad&utm_campaign=order_1001",
	}) {
		t.Fatalf("BodyLinkURLs: %v", got)
	}

	special, err := os.ReadFile("testdata/serialize/special.golden")
	if err != nil {
		t.Fatal(err)
	}
	if got := BodyPlainText(string(special)); got != `<script>alert("x")</script> & ещё`+"\nкод и ссылка с запросом" {
		t.Fatalf("BodyPlainText special: %q", got)
	}
	links := BodyLinkURLs(string(special) + "\n" + string(special))
	if !slices.Equal(links, []string{"https://a.test/?q=1&r=2&utm_source=max&utm_medium=channel_ad&utm_campaign=order_1001#top"}) {
		t.Fatalf("BodyLinkURLs special (deduplicated): %v", links)
	}
	if got := BodyLinkURLs("без ссылок"); len(got) != 0 {
		t.Fatalf("no links: %v", got)
	}
}
