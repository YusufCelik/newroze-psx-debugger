# Newroze Go Playstation Server
Playstation GDB debugger proxy written in GO

This is the Newroze server that needs to run for GDB to be able to talk with the Playstation. 

To install, just checkout the project, and type:

```go install```

(Obviously you first need to have golang installed: https://golang.org/dl/)

Afterwards you can specify a device/comport (serial->Playstation) and launch the debugger server.

For example:

```playstationgodebugger -device /dev/tty.usbserial-AC4N04E0```
