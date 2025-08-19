+++
date = "2019-08-26"
_created = 1566812441000
_updated = 1566815793000
site = "tech"
id = "b25b579f-e7cb-4ab6-9a71-6bd02775eb83"
title = "Automated Plant Watering System  - Part 1"
desc = ""
slug = "automating-watering-plants"
status = "published"
updated = "2019-08-26"
+++


<!--kg-card-begin: markdown-->
As I finally found sometime at hand, I decided to attend to one of my long standing projects to automate watering of my plants, which have been through stressful troughs of famine and crests of over-watering.

![DSC05728](/blog/content/images/2019/08/DSC05728.jpg)

![DSC05725](/blog/content/images/2019/08/DSC05725.jpg)

### Goals

One of the goals were that the system should last long enough while running off a battery. Secondary goals was the use of soil moisture sensors to optimize further.

### Bill of materials:

1. Digispark Attiny85
2. Common NPN Transistor
3. 3.7V LiPo
4. 3V DC Pump
5. Diode (or LED)
### Design:

Its a straight forward circuit with the MCU's output controlling the P of NPN Transistor. The Transistor serves to amplify the current as the DC motors are rated at 200mA while Attiny85 is only able to deliver few milliAmps of current.

I modified Digispark Attiny85 by disconnecting the onboard voltage regulator as it consumes ~1mA of current even when the MCU is in deep sleep. As Attiny85 is rated from 2.7v-5.5v the 3.7v LiPo's 4.1v output would be well within the required voltage range.

A DC Motor is an inductor - so when the motor is turned off, the stored energy in the motor flows back as reverse current. It can be powerful enough to damage the MCU, so a diode connected in reverse is needed. Due to lack of components I improvised by the use of a common LED connected in reverse.

![Circuit Diagram](/blog/content/images/2019/08/image.png)

### The Program

The MCU is programmed to wake up every second via a watchdog timer interrupt. On wake, it checks if it needs to turn on or turn off the output, performs it and goes back to sleep. This means the MCU is asleep even when the motor is running. The ON/OFF state is maintained with the count of wake up cycles rather than milliseconds - this works as each sleep cycle is 1 second.

Digispark's Attiny85's fuses are set at 16Mhz but the input is not exactly stable due the absense of voltage regulatr, this can lead to drift in timekeeping of few minutes per day.

```c
#include <avr/sleep.h>
#include <avr/interrupt.h>
#include <avr/wdt.h>

#define adc_disable() (ADCSRA &= ~(1<<ADEN)) // disable ADC (before power-off)
#define adc_enable()  (ADCSRA |=  (1<<ADEN)) // re-enable ADC

#define MPIN 1

unsigned int lpCntr=0;
unsigned int isOn=0;
const unsigned int onInterval = 2; // 2 sleep cycles
const unsigned int offInterval = 60 * 60; //(60 * 60 * 6) - onInterval; //round it off to 6 hours
const unsigned int repInterval = onInterval + offInterval;

void setup() {
  for(int i=0; i<6; i++) {
    pinMode(i, INPUT);
    digitalWrite(i, LOW);
  }

  adc_disable();
  wdt_reset();          //watchdog reset
  wdt_enable(WDTO_1S);  //1s watchdog timer
  WDTCR |= _BV(WDIE);   //Interrupts watchdog enable
  sei();                //enable interrupts
  set_sleep_mode(SLEEP_MODE_PWR_DOWN);
}

void loop() {

  // Loop counter reset
  if( lpCntr >= repInterval ) {
    lpCntr = lpCntr % repInterval;
  }

  // Loop counter began, so write high
  if( lpCntr == 0 ) {
    pinMode(MPIN, OUTPUT);
    digitalWrite(MPIN, HIGH);
    isOn=1;
  } 
  else
  if( isOn == 1 && lpCntr >= onInterval ) {
    // Reset the output to LOW if we are done
    pinMode(MPIN, OUTPUT);
    digitalWrite(MPIN, LOW);
    isOn=0;
  }
  
  lpCntr++;

  sleep_enable();
  sleep_cpu();

}

ISR (WDT_vect) {
  WDTCR |= _BV(WDIE);
}
```

<!--kg-card-end: markdown-->




	
