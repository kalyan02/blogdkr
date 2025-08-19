+++
date = "2017-07-26"
_created = 1501096084000
_updated = 1522453829000
site = "tech"
id = "bae55c2d-20fb-4078-85e3-9c6663d1f6ed"
title = "Getting started with a blank iOS Project"
desc = ""
slug = "blank-ios-project-without-storyboards"
status = "published"
updated = "2018-03-30"
+++


Everytime I start a new Xcode/iOS project, there are few rituals I perform, quite religiously.

## Disable Storyboards

I dislike storyboards and xibs with passion. Fiddling around a clunky Xcode interface to get layouts to work, with uncertainty of how they will behave on different resolutions, is quite annoying. And in large team settings which I worked in before, its quite frankly useless.

What's needed for doing it?

1. Delete files `Main.storyboard` and `Main-iPad.storyboard`
2. Delete the `"Main Interface"` setting found in
    - `Project Settings`
    - `Targets (select)`
    - `General`
    - `Deployment Info`
    - `Main Interface`.

## Setup the AppDelegate

Now that the storyboards are gone, we need to configure the app delegate as to what view controller is to be shown

I often end up doing this

**AppDelegate.m**

```m
- (BOOL)application:(UIApplication *)application didFinishLaunchingWithOptions:(NSDictionary *)launchOptions {

    // Instantiate the VC    
    MainViewController *vc = [MainViewController new];
    // Instantiate the navigation bar
    UINavigationController *nav = [UINavigationController alloc] initWithRootViewController:vc];

    // Instantiate window
    self.window = [[UIWindow alloc] initWithFrame:[UIScreen mainScreen].bounds];
    // Set the root vc
    self.window.rootViewController = nav;
    
    // make the window visible
    [self.window makeKeyAndVisible];
    return YES;
}

```

## Solid UINavigationBar

I find the the default navigation bar that comes along with translucent background useless. Majority of all apps end up either with a solid color or a subtle gradient.

Sometimes I prefer to subclass `UINavigationController` and other times, I just configure it directly. Either case the essential snippet of code is this:

```m
// Disable translucency 
navViewController.navigationBar.translucent = NO;
// Set it to something jazzy
navViewController.navigationBar.barTintColor = [UIColor orangeColor];
```

## Setup MainViewController

When a view controller is added to a UINavigationController stack, it doesn't flow from below the navigation bar, but from underneath it (from iOS 7~). This again seems quite pointless.

The sane default I follow is to set the `edgesForExtendedLayout` property

**MainViewController -viewDidLoad**

```m
self.edgesForExtendedLayout = UIRectEdgeNone;
```

## Misc

**Auto Layout**:


Although it tends to be more verbose, I prefer writing code that declares how interface objects look and behave proportional to the screen size, than use Storyboards or XIBs.

**Manual Layout**:


Sometimes I don't mind setting the frames manually for simpler things.

**Libraries**:


Most times I end up adding these as well

* Flurry and Google analytics for analytics/tracking
* Additional classes to enable use of a file based, simple key value store.
* Shorthand functions Eg: `UIColor* RGBColor(r,g,b)`
* UI Utilities that allow me to do this `view.top = 20;`
More on these later.




	
