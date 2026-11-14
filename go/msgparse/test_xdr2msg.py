# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

import unittest

from xdr2msg import Gen, Parser


class XdrOctalTest(unittest.TestCase):
    def test_octal_constant_and_named_array_bound(self):
        msg = Gen(Parser('const SIZE = 010; struct Blob { opaque bytes[SIZE]; };').parse()).emit()

        self.assertIn('const SIZE = 8;', msg)
        self.assertIn('u8 bytes[8];', msg)

    def test_inline_octal_array_bound(self):
        msg = Gen(Parser('struct Blob { opaque bytes[010]; };').parse()).emit()

        self.assertIn('u8 bytes[8];', msg)

    def test_zero_decimal_and_hex_literals_keep_their_spelling(self):
        msg = Gen(Parser('const ZERO = 0; const DEC = 10; const HEX = 0x10;').parse()).emit()

        self.assertIn('const ZERO = 0;', msg)
        self.assertIn('const DEC = 10;', msg)
        self.assertIn('const HEX = 0x10;', msg)


if __name__ == '__main__':
    unittest.main()
