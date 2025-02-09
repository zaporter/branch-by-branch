import Corelib
--R: DataNatBasic.lean 100

namespace Test_DataNatBinaryRec

variable {motive : Nat → Sort u}
universe u


open Nat

--T: bit
/-- `bit b` appends the digit `b` to the binary representation of its natural number input. -/
example (b : Bool) (n:Nat): bit b n = cond b (2 * n + 1) (2 * n) := by
  cases b <;> simp [bit]
--E
--G: bit_decide_mod_two_eq_one_shiftRight_one
example (n : Nat) : bit (n % 2 = 1) (n >>> 1) = n := by exact?
--E
--G: bit_testBit_zero_shiftRight_one
example (n : Nat) : bit (n.testBit 0) (n >>> 1) = n := by exact?
--E
--G: bit_eq_zero_iff
example {n : Nat} {b : Bool} : bit b n = 0 ↔ n = 0 ∧ b = false := by exact?
--E
--T: bitCasesOn
/-- Test that bitCasesOn correctly decomposes a number into its binary representation -/
example {n : Nat} (h : ∀ b n, motive (bit b n)) :
    bitCasesOn n h = congrArg motive n.bit_testBit_zero_shiftRight_one ▸ h (1 &&& n != 0) (n >>> 1) := by exact?
--E
--G: bitCasesOn_bit
example (h : ∀ b n, motive (bit b n)) (b : Bool) (n : Nat) :
    bitCasesOn (bit b n) h = h b n := by exact?
--E
--T: binaryRec
example {z : motive 0} {f : ∀ b n, motive n → motive (bit b n)} {n : Nat} :
    binaryRec z f n = if n0 : n = 0 then congrArg motive n0 ▸ z
      else let x := f (1 &&& n != 0) (n >>> 1) (binaryRec z f (n >>> 1));
           congrArg motive n.bit_testBit_zero_shiftRight_one ▸ x := by exact?
--E
--NOTE: Skipping binaryRecFromOne and BinaryRec' because I am having trouble with the types
--G: bit_val
example (b n) : bit b n = 2 * n + b.toNat := by exact?
--E
--G: bit_div_two
example (b n) : bit b n / 2 = n := by exact?
--E
--G: bit_mod_two
example (b n) : bit b n % 2 = b.toNat := by exact?
--E
--G: bit_shiftRight_one
example (b n) : bit b n >>> 1 = n := by exact?
--E
--G: testBit_bit_zero
example (b n) : (bit b n).testBit 0 = b := by exact?
--E
--G: bitCasesOn_bit
example (h : ∀ b n, motive (bit b n)) (b : Bool) (n : Nat) :
    bitCasesOn (bit b n) h = h b n := by
  exact?
--E
--G: binaryRec_zero
example (z : motive 0) (f : ∀ b n, motive n → motive (bit b n)) :
    binaryRec z f 0 = z := by
  exact?
--E
--G: binaryRec_one
example (z : motive 0) (f : ∀ b n, motive n → motive (bit b n)) :
    binaryRec (motive := motive) z f 1 = f true 0 z := by exact?
--E
--G: binaryRec_eq
example {z : motive 0} {f : ∀ b n, motive n → motive (bit b n)}
    (b n) (h : f false 0 z = z ∨ (n = 0 → b = true)) :
    binaryRec z f (bit b n) = f b n (binaryRec z f n) := by
  exact?
--E
