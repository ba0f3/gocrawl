package main

import (
	"fmt"
	"net/url"
)

func main() {
    base, _ := url.Parse("https://user:pass@example.com/foo")
    fmt.Printf("orig root rel -> %q\n", base.ResolveReference(&url.URL{Path: "/root/path"}).String())

    userinfo := ""
    if base.User != nil {
        userinfo = base.User.String() + "@"
    }
    fast := base.Scheme + "://" + userinfo + base.Host + "/root/path"
    fmt.Printf("fast root rel -> %q\n", fast)
}
