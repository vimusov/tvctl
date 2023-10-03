#include <Arduino.h>
#include <IRremote.h>

IRrecv receiver(2, ENABLE_LED_FEEDBACK);

void setup()
{
	Serial.begin(9600);
	receiver.enableIRIn();
}

void loop()
{
	if (receiver.decode()) {
		Serial.println(receiver.decodedIRData.command);
		receiver.resume();
		delay(250);
	}
}
