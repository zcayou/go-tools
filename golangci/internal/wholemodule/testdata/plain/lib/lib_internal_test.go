package lib

import "net/url"

func greeting() string { return url.PathEscape(Hello()) }
