package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/nip19"
)

const name = "nostr-fushinsha-summary"

const version = "0.0.0"

var revision = "HEAD"

const feedURL = "https://nordot.app/-/feed/posts/rss?unit_id=133089874031904245"

var (
	relays = []string{
		"wss://yabu.me",
	}
	tt bool
)

type rank struct {
	name  string
	count int
}

func postRanks(ctx context.Context, ms nostr.MultiStore, nsec string, ranks []rank) error {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s の不審者情報まとめ\n\n", time.Now().Format("2006/01/02"))
	for _, rank := range ranks {
		fmt.Fprintf(&buf, "%s %d 件\n",
			rank.name,
			rank.count,
		)
	}
	fmt.Fprintln(&buf, "\n#fushinsha")

	if tt {
		io.Copy(os.Stdout, &buf)
		return nil
	}

	eev := nostr.Event{}
	var sk string
	if _, s, err := nip19.Decode(nsec); err == nil {
		sk = s.(string)
	} else {
		return err
	}
	if pub, err := nostr.GetPublicKey(sk); err == nil {
		if _, err := nip19.EncodePublicKey(pub); err != nil {
			return err
		}
		eev.PubKey = pub
	} else {
		return err
	}

	eev.Content = buf.String()
	eev.CreatedAt = nostr.Now()
	eev.Kind = nostr.KindTextNote
	eev.Tags = eev.Tags.AppendUnique(nostr.Tag{"t", "fushinsha"})
	eev.Sign(sk)

	return ms.Publish(ctx, eev)
}

func init() {
	time.Local = time.FixedZone("Local", 9*60*60)
}

func main() {
	var ver bool
	flag.BoolVar(&ver, "version", false, "show version")
	flag.BoolVar(&tt, "t", false, "test")
	flag.Parse()

	if ver {
		fmt.Println(version)
		os.Exit(0)
	}

	ms := nostr.MultiStore{}
	ctx := context.TODO()
	for _, r := range relays {
		rr, err := nostr.RelayConnect(ctx, r)
		if err == nil {
			ms = append(ms, rr)
		}
	}

	feed, err := gofeed.NewParser().ParseURL(feedURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	m := map[string]int{}
	pat := regexp.MustCompile(`^（([^）]+)）`)
	for _, item := range feed.Items {
		ss := pat.FindStringSubmatch(item.Title)
		if len(ss) < 2 {
			continue
		}
		m[ss[1]]++
	}
	var ranks []rank
	for k, v := range m {
		ranks = append(ranks, rank{name: k, count: v})
	}
	sort.Slice(ranks, func(i, j int) bool {
		return ranks[i].count > ranks[j].count
	})
	if len(ranks) == 0 {
		return
	}

	ctx = context.TODO()
	postRanks(ctx, ms, os.Getenv("BOT_NSEC"), ranks)
}
