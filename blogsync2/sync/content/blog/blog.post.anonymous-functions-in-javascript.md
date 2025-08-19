+++
date = "2012-02-14"
_created = 1329214496000
_updated = 1522453829000
site = "tech"
id = "adc45e98-abd9-463e-ac0a-d154a323112f"
title = "Anonymous functions in Javascript"
desc = ""
slug = "anonymous-functions-in-javascript"
status = "published"
updated = "2018-03-30"
+++


The usual way to create *anonymous functions* is to write something like this

```js
(function() {
 alert('hello');
})();   
```

I recently learnt this works too (#1)

```js
!function() {
 alert('hello');
}(); 
```

But interestingly enough that same code, without the !, throws a syntax error. (#2)

```js
function() {
 alert('hello');
}(); 
```

This had me stoked for a while until I realized why and it was obvious all along.


In the first case the "!" makes the function be treated as an anonymous function object and then negating it to result false after it has executed. But in the #2, the absence of an expression and starting of the statement with 'function' keyword makes the interpreter look for a named function, which it wouldn't qualify for due to the absence of a name, thereby resulting in syntax error.

The same piece of code in the context of an expression, works just fine

```js
> x = function() { return 10; }();
> x
 10
```

So, will naming work? Yes.

```js
//Naming works too
> x = function xyz() { return 15; }();
> x
 15
```

Can we call it outside an expression without grouping? No. That would throw an error, as after consuming a fully formed function, the interpreter tries to consider the empty anonymous parenthesis as a different statement.

```js
> function xyz() { return 15; }();  //error
> ();  //error
```

Make the empty parenthesis a valid statement, and it works

```js
> function xyz() { return 15; }(1);
```

How do we know its an independent statement?

```js
> function xyz() { return 15; }(console.log(20));
> 20
```

But then how do we call it ? By Grouping.

```js
> (function xyz() { return 15; })(console.log(20));
> 20  //console log
<- 15  //return
```

Why does it work? The function instead of being a simple statement, now is an object in an expression, which means can be evaluated and can take arguments. But arguments need not be an expression and can be empty. That brings us back to the standard statement.

```js
> (function xyz() { return 15; })();
<- 15  //return
```




	
