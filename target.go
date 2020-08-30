package main

// TargetDescriptionXML describes the Playstation target to GDB
var TargetDescriptionXML string = `<?xml version="1.0"?>
<!DOCTYPE feature SYSTEM "gdb-target.dtd">
<target version="1.0">
<!-- Helping GDB -->
<architecture>mips:3000</architecture>
<osabi>none</osabi>
<!-- Mapping ought to be flexible, but there seems to be some
     hardcoded parts in gdb, so let's use the same mapping. -->
<feature name="org.gnu.gdb.mips.cpu">
  <reg name="r0" bitsize="32" regnum="0"/>
  <reg name="r1" bitsize="32" regnum="1"/>
  <reg name="r2" bitsize="32" regnum="2"/>
  <reg name="r3" bitsize="32" regnum="3"/>
  <reg name="r4" bitsize="32" regnum="4"/>
  <reg name="r5" bitsize="32" regnum="5"/>
  <reg name="r6" bitsize="32" regnum="6"/>
  <reg name="r7" bitsize="32" regnum="7"/>
  <reg name="r8" bitsize="32" regnum="8"/>
  <reg name="r9" bitsize="32" regnum="9"/>
  <reg name="r10" bitsize="32" regnum="10"/>
  <reg name="r11" bitsize="32" regnum="11"/>
  <reg name="r12" bitsize="32" regnum="12"/>
  <reg name="r13" bitsize="32" regnum="13"/>
  <reg name="r14" bitsize="32" regnum="14"/>
  <reg name="r15" bitsize="32" regnum="15"/>
  <reg name="r16" bitsize="32" regnum="16"/>
  <reg name="r17" bitsize="32" regnum="17"/>
  <reg name="r18" bitsize="32" regnum="18"/>
  <reg name="r19" bitsize="32" regnum="19"/>
  <reg name="r20" bitsize="32" regnum="20"/>
  <reg name="r21" bitsize="32" regnum="21"/>
  <reg name="r22" bitsize="32" regnum="22"/>
  <reg name="r23" bitsize="32" regnum="23"/>
  <reg name="r24" bitsize="32" regnum="24"/>
  <reg name="r25" bitsize="32" regnum="25"/>
  <reg name="r26" bitsize="32" regnum="26"/>
  <reg name="r27" bitsize="32" regnum="27"/>
  <reg name="r28" bitsize="32" regnum="28"/>
  <reg name="r29" bitsize="32" regnum="29"/>
  <reg name="r30" bitsize="32" regnum="30"/>
  <reg name="r31" bitsize="32" regnum="31"/>
  <reg name="lo" bitsize="32" regnum="33"/>
  <reg name="hi" bitsize="32" regnum="34"/>
  <reg name="pc" bitsize="32" regnum="37"/>
</feature>
<feature name="org.gnu.gdb.mips.cp0">
  <reg name="status" bitsize="32" regnum="32"/>
  <reg name="badvaddr" bitsize="32" regnum="35"/>
  <reg name="cause" bitsize="32" regnum="36"/>
  <reg name="dcic" bitsize="32" regnum="38"/>
  <reg name="bpc" bitsize="32" regnum="39"/>
  <reg name="tar" bitsize="32" regnum="40"/>
</feature>
<!-- We don't have an FPU, but gdb hardcodes one, and will choke
     if this section isn't present. -->
<feature name="org.gnu.gdb.mips.fpu">
  <reg name="f0" bitsize="32" type="ieee_single" regnum="41"/>
  <reg name="f1" bitsize="32" type="ieee_single"/>
  <reg name="f2" bitsize="32" type="ieee_single"/>
  <reg name="f3" bitsize="32" type="ieee_single"/>
  <reg name="f4" bitsize="32" type="ieee_single"/>
  <reg name="f5" bitsize="32" type="ieee_single"/>
  <reg name="f6" bitsize="32" type="ieee_single"/>
  <reg name="f7" bitsize="32" type="ieee_single"/>
  <reg name="f8" bitsize="32" type="ieee_single"/>
  <reg name="f9" bitsize="32" type="ieee_single"/>
  <reg name="f10" bitsize="32" type="ieee_single"/>
  <reg name="f11" bitsize="32" type="ieee_single"/>
  <reg name="f12" bitsize="32" type="ieee_single"/>
  <reg name="f13" bitsize="32" type="ieee_single"/>
  <reg name="f14" bitsize="32" type="ieee_single"/>
  <reg name="f15" bitsize="32" type="ieee_single"/>
  <reg name="f16" bitsize="32" type="ieee_single"/>
  <reg name="f17" bitsize="32" type="ieee_single"/>
  <reg name="f18" bitsize="32" type="ieee_single"/>
  <reg name="f19" bitsize="32" type="ieee_single"/>
  <reg name="f20" bitsize="32" type="ieee_single"/>
  <reg name="f21" bitsize="32" type="ieee_single"/>
  <reg name="f22" bitsize="32" type="ieee_single"/>
  <reg name="f23" bitsize="32" type="ieee_single"/>
  <reg name="f24" bitsize="32" type="ieee_single"/>
  <reg name="f25" bitsize="32" type="ieee_single"/>
  <reg name="f26" bitsize="32" type="ieee_single"/>
  <reg name="f27" bitsize="32" type="ieee_single"/>
  <reg name="f28" bitsize="32" type="ieee_single"/>
  <reg name="f29" bitsize="32" type="ieee_single"/>
  <reg name="f30" bitsize="32" type="ieee_single"/>
  <reg name="f31" bitsize="32" type="ieee_single"/>
  <reg name="fcsr" bitsize="32" group="float"/>
  <reg name="fir" bitsize="32" group="float"/>
</feature>
</target>`
