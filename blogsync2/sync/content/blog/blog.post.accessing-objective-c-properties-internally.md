+++
date = "2014-06-13"
_created = 1402617600000
_updated = 1522453829000
site = "tech"
id = "817539c8-eafb-4363-864c-a0293437715a"
title = "Accessing Objective-C properties internally"
desc = ""
slug = "accessing-objective-c-properties-internally"
status = "published"
updated = "2018-03-30"
+++


Inside an objective class, there are 2 ways to access any declared property

* `self.propertyName`
* `_propertyName`
Although both the uses hypothetically have the similar effect, there are 3 distinct differences

1. `self.propertyName` trigggers the setter, which is important if you are performing any additional operations or validations. This does not happen when accessing the ivar directly


2. `self.propertyName` also triggers **KVO Notifications** which _propertyName conveniently ignores.


3. Accesing property also gives access to various memory management features for free. Eg: a property with `copy` attribute will enforce it by creating a copy of the instance being assigned rather than just assigning the pointer, which is particularly useful when assinging mutable data to immutable property type.
### Example:

```m
@interface ViewController ()
@property (nonatomic, strong) NSString *theName;
@end

- (void)viewDidLoad 
{
    [super viewDidLoad];
	    
    [self addObserver:self
           forKeyPath:@"theName"
              options:NSKeyValueObservingOptionNew
              context:nil];

    self.theName = @"Hello world";	// Triggers KVO
    _theName = @"Fancy world";		// No KVO

}

- (void)observeValueForKeyPath:(NSString *)keyPath
                      ofObject:(id)object
                        change:(NSDictionary *)change
                       context:(void *)context
{
    NSLog(@"Key path changed - %@", change);
}
```

### output

Notice only one of the 2 assignments triggers KVO notification

```bash
2014-06-13 06:03:56.163 NotifTest[24710:1356234] Key path changed - {
    kind = 1;
    new = "Hello world";
}
```




	
