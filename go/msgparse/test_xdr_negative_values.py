# SPDX-License-Identifier: Apache-2.0 WITH LLVM-exception

import struct
import unittest
from pathlib import Path

from xdr2msg import Gen, Parser, split_inline_arrays, tokenize


class NegativeXdrValuesTest(unittest.TestCase):
    def test_negative_constant_is_not_emitted_as_positive(self):
        with self.assertRaises(SyntaxError):
            Parser('const INVALID = -1;').parse()

    def test_negative_enum_value_preserves_signed_wire_bits(self):
        output = Gen(Parser('enum Status { ERROR = -1, OK = 0 };').parse()).emit()
        self.assertIn('ERROR = 4294967295;', output)
        self.assertEqual(struct.pack('>I', 4294967295), struct.pack('>i', -1))

    def test_negative_union_label_is_not_emitted_as_positive(self):
        with self.assertRaises(SyntaxError):
            Parser('union Choice switch (int tag) { case -1: void; };').parse()

    def test_minus_inside_comments_and_strings_is_not_a_negative_value(self):
        self.assertEqual(list(tokenize('/* -1 */ // -2\n "-3"')), [('STR', '"-3"')])

    def test_nonnegative_constants_keep_their_values(self):
        output = Gen(Parser('const ZERO = 0; const VALUE = 10; const HEX = 0x10;').parse()).emit()
        self.assertIn('const ZERO = 0;', output)
        self.assertIn('const VALUE = 10;', output)
        self.assertIn('const HEX = 0x10;', output)

    def test_int32_minimum_is_encoded_without_losing_sign(self):
        output = Gen(Parser('enum Status { MINIMUM = -2147483648 };').parse()).emit()
        self.assertIn('MINIMUM = 2147483648;', output)
        self.assertEqual(struct.pack('>I', 2147483648), struct.pack('>i', -2147483648))

    def test_negative_enum_outside_int32_is_rejected(self):
        with self.assertRaisesRegex(ValueError, 'out of int32 range'):
            Gen(Parser('enum Status { INVALID = -2147483649 };').parse()).emit()

    def test_bundled_recall_mask_values_keep_their_wire_bits(self):
        output = Gen(Parser('enum ff_cb_recall_any_mask { FF_RCA4_TYPE_MASK_READ = -2, FF_RCA4_TYPE_MASK_RW = -1 };').parse()).emit()
        self.assertIn('FF_RCA4_TYPE_MASK_READ = 4294967294;', output)
        self.assertIn('FF_RCA4_TYPE_MASK_RW   = 4294967295;', output)

    def test_bundled_nfs_schema_matches_generator(self):
        directory = Path(__file__).resolve().parents[1] / 'nfsd' / 'internal' / 'nfs4'
        source = (directory / 'nfs.x').read_text()
        output = Gen(split_inline_arrays(Parser(source).parse())).emit()
        self.assertEqual(output, (directory / 'nfs.msg').read_text())


if __name__ == '__main__':
    unittest.main()
