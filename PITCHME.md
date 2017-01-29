#HSLIDE

## AnyCable
### A polyglot replacement for <span style="color:#e49436">ActionCable</span> server

#HSLIDE

## ActionCable

#### Easy to use <!-- .element: class="fragment" -->

#### Allows you to access business logic  <!-- .element: class="fragment" -->

#### Has JS client that just works <!-- .element: class="fragment" -->

#HSLIDE

## ActionCable

### is good for [designing live features](http://weblog.rubyonrails.org/2016/6/30/Rails-5-0-final/)


#HSLIDE

## But...
### is it ready for <span style="color:#e49436">production</span>?

#HSLIDE

## Benchmarks

#### Unfortunately, <span style="color:#e49436">ActionCable</span> leaves much to be desired

<span style="font-size:0.6em; color:gray">Press Down key to see charts and gifs</span>

#VSLIDE

## Memory

![memory](assets/Memory3.png)

#VSLIDE

## CPU

![cpu](assets/cpu_chart.gif)

#VSLIDE

## Broadcast Round Trip Time

![rtt](assets/RTT3.png)

#HSLIDE

### Let's extract <span style="color:#e49436">WebSockets</span> somewhere else!

#HSLIDE

## AnyCable

#### Combines the good parts from <span style="color:#e49436">ActionCable</span> with the power of your favorite language for concurrent applications

<span style="font-size:0.6em; color:gray">How it works? See below</span>

#VSLIDE

## How AnyCable Works

![diagram](assets/Scheme2.png)

#VSLIDE

## [gRPC](http://grpc.io)

### Makes AnyCable to be a <span style="color:#e49436">polyglot</span>

#VSLIDE

## AnyCable

#### [Compatible](https://github.com/anycable/anycable#actioncable-compatibility) with ActionCable (channels, javascript, broadcasting)

#### You can still use ActionCable for <span style="color:#e49436">development</span> and <span style="color:#e49436">testing</span>

#VSLIDE

## AnyCable Servers

- [anycable-go](https://github.com/anycable/anycable-go)

- [erlycable](https://github.com/anycable/erlycable)

#VSLIDE

## AnyCable

### [Demo Application](https://github.com/anycable/anycable_demo)

#HSLIDE

## Benchmarks Again

#### AnyCable shows much more better performance.

<span style="font-size:0.6em; color:gray">Press Down key to see charts and gifs</span>

#VSLIDE

## Memory

![memory](assets/Memory5.png)

#VSLIDE

## CPU

![cpu](assets/cpu_chart2.gif)

#VSLIDE

## Broadcast Round Trip Time

![rtt](assets/RTT5.png)


#HSLIDE

## Let's Make ActionCable Not Suck!

[anycable.evilmartians.io](http://anycable.io/)

Vladimir Dementyev [@palkan_tula](http://twitter.com/palkan_tula)

[Evil Martians](http://evilmartians.com)

Twitter [@any_cable](http://twitter.com/any_cable)

GitHub [@anycable](http://github.com/anycable)
