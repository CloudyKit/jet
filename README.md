# Jet Template Engine for GO [![Build Status](https://travis-ci.org/CloudyKit/jet.svg?branch=master)](https://travis-ci.org/CloudyKit/jet)

[![Join the chat at https://gitter.im/CloudyKit/jet](https://badges.gitter.im/CloudyKit/jet.svg)](https://gitter.im/CloudyKit/jet?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge)

Jet is template engine developed to be easy to use, powerful, dynamic and very fast.
Very fast without pre-compilation and dynamic features, yes this is possible.

Support template inheritance, with extends, and block imports, fast writer, auto escapee, and many
more features.

Jet is based on text/template, i used part of the lexer and part of the parser code.

#### Intellij Plugin

If you use intellij there is a plugin available in https://github.com/jhsx/GoJetPlugin
There is also a very good Go plugin for intellij ("https://github.com/go-lang-plugin-org/go-lang-idea-plugin")
GoJetPlugin + Go-lang-idea-plugin = Happiness :D


#### Features

* Template Inheritance
* File based
* Auto-escape
* Complex Expression
* Tests and Benchmarks
* Fast and light
* Simple syntax and simple to use
* Descriptive error messages with filename and line while parsing or evaluating templates

### Examples

#### Simple

```HTML

Hey {{name}}!
No Escape {{ "<a href=\""\">Link</a>" |unsafe}}

```

#### Extends

```HTML
{{extends "common/layout.jet"}}


{{block Content}}
	{{.PageHeader}}
    {* this is a comment *}
    {{.PageContent |unsafe}}
{{end}}
```


#### Range

```HTML
{{extends "common/layout.jet"}}


{{block Content}}
	{{.PageHeader}}

    {{range .Result}}
    	<div>
        	<div class="result-header">{{.Title}}</div>
            <div class="result-description">{{.Description}}
            <a href="{{.Linkme()}}">Read more</a></div>
        </div>
    {{end}}
{{end}}
```

#### Import Extends Yield

```HTML
{{extends "common/layout.jet"}}
{{import "common/menu.jet"}}

{{block Header}}
<!-- this block commes from "common/menu.jet" -->
<nav class="nav">{{yield menu}}</nav>
{{end}}

{{block Content}}
	{{.PageHeader}}

    {{range .Result}}
    	<div>
        	<div class="result-header">{{.Title}}</div>
            <div class="result-description">{{.Description}}
            <a href="{{.Linkme()}}">Read more</a></div>
        </div>
    {{end}}
{{end}}
```
#### Faster than some pre-compiled template engines

Benchmark consist of range over a slice of data printing the values,
the benchmark is based on "https://github.com/SlinSo/goTemplateBenchmark",

Jet performs better than all template engines without pre-compilation, and peforms better than gorazor,
Ftmpl, Egon which are pre-compiled to go.


###### Benchmarks

go 1.6.2
```
PASS
BenchmarkEgonSlinso-4      	 2000000	       989 ns/op	     517 B/op	       0 allocs/op
BenchmarkQuicktemplate-4   	 1000000	      1004 ns/op	     999 B/op	       0 allocs/op
BenchmarkEgo-4             	 1000000	      2137 ns/op	     603 B/op	       8 allocs/op

BenchmarkJet-4             	  500000	      2798 ns/op	     536 B/op	       0 allocs/op
BenchmarkJetHTML-4         	  500000	      2822 ns/op	     536 B/op	       0 allocs/op

BenchmarkGorazor-4         	  500000	      3028 ns/op	     613 B/op	      11 allocs/op
BenchmarkFtmpl-4           	  500000	      3192 ns/op	    1152 B/op	      12 allocs/op
BenchmarkEgon-4            	  300000	      4673 ns/op	    1172 B/op	      22 allocs/op
BenchmarkKasia-4           	  200000	      6902 ns/op	    1789 B/op	      26 allocs/op
BenchmarkSoy-4             	  200000	      7144 ns/op	    1684 B/op	      26 allocs/op
BenchmarkMustache-4        	  200000	      8213 ns/op	    1568 B/op	      28 allocs/op
BenchmarkPongo2-4          	  200000	      9989 ns/op	    2949 B/op	      46 allocs/op
BenchmarkGolang-4          	  100000	     16284 ns/op	    2039 B/op	      38 allocs/op
BenchmarkAmber-4           	  100000	     17208 ns/op	    2050 B/op	      39 allocs/op
BenchmarkHandlebars-4      	   50000	     29864 ns/op	    4258 B/op	      90 allocs/op
BenchmarkAce-4             	   30000	     40771 ns/op	    5710 B/op	      77 allocs/op
BenchmarkDamsel-4          	   20000	     95947 ns/op	   11160 B/op	     165 allocs/op
ok  	github.com/SlinSo/goTemplateBenchmark	34.384s
```

go tip
```
BenchmarkQuicktemplate-4      	 2000000	       916 ns/op	     999 B/op	       0 allocs/op
BenchmarkEgonSlinso-4         	 2000000	      1074 ns/op	     517 B/op	       0 allocs/op
BenchmarkEgo-4                	 1000000	      1822 ns/op	     603 B/op	       8 allocs/op

BenchmarkJetHTML-4            	  500000	      2627 ns/op	     536 B/op	       0 allocs/op
BenchmarkJet-4                	  500000	      2652 ns/op	     536 B/op	       0 allocs/op

BenchmarkFtmpl-4              	  500000	      2700 ns/op	    1152 B/op	      12 allocs/op
BenchmarkGorazor-4            	  500000	      2858 ns/op	     613 B/op	      11 allocs/op
BenchmarkEgon-4               	  500000	      4023 ns/op	     827 B/op	      22 allocs/op
BenchmarkSoy-4                	  300000	      5590 ns/op	    1784 B/op	      26 allocs/op
BenchmarkKasia-4              	  200000	      6487 ns/op	    1789 B/op	      26 allocs/op
BenchmarkMustache-4           	  200000	      6515 ns/op	    1568 B/op	      28 allocs/op
BenchmarkPongo2-4             	  200000	      7602 ns/op	    2949 B/op	      46 allocs/op
BenchmarkAmber-4              	  100000	     13942 ns/op	    2050 B/op	      39 allocs/op
BenchmarkGolang-4             	  100000	     16945 ns/op	    2039 B/op	      38 allocs/op
BenchmarkHandlebars-4         	  100000	     20152 ns/op	    4258 B/op	      90 allocs/op
BenchmarkAce-4                	   50000	     33091 ns/op	    5509 B/op	      77 allocs/op
BenchmarkDamsel-4             	   20000	     86340 ns/op	   11159 B/op	     165 allocs/op
PASS
ok  	github.com/SlinSo/goTemplateBenchmark	36.200s
```

#### Contributing

Any contribution is welcome, if you found a bug please report in the issues tracker.

#### Thanks

-   @golang developers, for the awesome language, and std library
-   @SlinSo for the benchmarks that i used as base to show the results above
