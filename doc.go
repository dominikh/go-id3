/*
Foo bar.

Supported versions

This library supports reading v2.3 and v2.4 tags, but only writing
v2.4 tags.

The primary reason for not allowing writing older versions is that
they cannot represent all data that is available with v2.4, and
designing the API in a way that's both user friendly and able to
reject data is not worth the trouble.

Automatic upgrading

The library's internal representation of tags matches that of v2.4.
When tags with an older version are being read, they will be
automatically converted to v2.4.

One consequence of this is that when you read a file with v2.3 tags
and immediately save it, it will now be a file with valid v2.4 tags.

The upgrade process makes following changes to the tags:

  - TYER, TDAT and TIME get replaced by TDRC
  - TORY gets replaced by TDOR
  - XDOR gets replaced by TDOR
  - The slash as a separator for multiple values gets replaced by null bytes

One special case is the TRDA frame because there is no way to
automatically convert it to v2.4. The upgrade process will not
directly delete the frame, so that you can manually upgrade it if
desired, but it won't be written back to the file. In reality, the frame
should be both rarely used and insignifcant enough to be a big loss.


Accessing and manipulating frames

There are two ways to access frames: Using provided getter and setter
methods (there is one for every standard frame), and working directly
with the underlying frames.

For frames that usually support multiple values, e.g. languages, there
will be two different setters and getters: One that operates on slices
and one that operates on single values. When getting a single value,
it will return the first value from the underlying list. When setting
a single value, it will overwrite the list with a single value.

Text frames and user text frames can be manipulated with the
GetTextFrame* and SetTextFrame* class. There are special methods for
working with integers, slices and times. This class of functions
expects the raw frame names (e.g. "TLEN"), with the special case of
user text frames ("TXXX") where it expects a format of the kind
"TXXX:The frame description" to address a specific user text frame.

*/
package id3
