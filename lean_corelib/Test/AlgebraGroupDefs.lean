import Corelib
--R: DataNatBinaryRec.lean 100
--R: AlgebraGroupZeroOne.lean 100
--R: AlgebraGroupOperations.lean 100
--R: LogicFunctionDefs.lean 100

namespace Test_AlgebraGroupDefs

universe u

variable {G : Type*}

open Function

--T: leftMul
example [Mul G] (g : G) (x : G) : leftMul g x = g * x := by exact?
--E
--T: rightMul
example [Mul G] (g : G) (x : G) : rightMul g x = x * g := by exact?
--E
--T: IsLeftCancelMul
example [Mul G] [IsLeftCancelMul G] (a b c : G) :
    a * b = a * c → b = c := IsLeftCancelMul.mul_left_cancel a b c
--E
--T: IsRightCancelMul
example [Mul G] [IsRightCancelMul G] (a b c : G) :
    a * b = c * b → a = c := IsRightCancelMul.mul_right_cancel a b c
--E
--T: IsCancelMul
example [Mul G] [IsCancelMul G] : IsLeftCancelMul G ∧ IsRightCancelMul G :=
  ⟨inferInstance, inferInstance⟩
--E
--T: IsLeftCancelAdd
example [Add G] [IsLeftCancelAdd G] (a b c : G) :
    a + b = a + c → b = c := IsLeftCancelAdd.add_left_cancel a b c
--E
--T: IsRightCancelAdd
example [Add G] [IsRightCancelAdd G] (a b c : G) :
    a + b = c + b → a = c := IsRightCancelAdd.add_right_cancel a b c
--E
--T: IsCancelAdd
example [Add G] [IsCancelAdd G] : IsLeftCancelAdd G ∧ IsRightCancelAdd G :=
  ⟨inferInstance, inferInstance⟩
--E
--G: mul_left_cancel
section Test_IsLeftCancelMul
variable [Mul G] [IsLeftCancelMul G] {a b c : G}
--NOTE: Something is broken here. Exact wont find the goal.
example : a * b = a * c → b = c := by exact mul_left_cancel
example : a * b = a * c ↔ b = c := by exact?
end Test_IsLeftCancelMul
--E
--G: add_left_cancel
section Test_IsLeftCancelAdd
variable [Add G] [IsLeftCancelAdd G] {a b c : G}
--NOTE: Something is broken here. Exact wont find the goal.
example : a + b = a + c → b = c := by exact add_left_cancel
example : a + b = a + c ↔ b = c := by exact?
end Test_IsLeftCancelAdd
--E
--G: mul_right_cancel
section Test_IsRightCancelMul
variable [Mul G] [IsRightCancelMul G] {a b c : G}
--NOTE: Something is broken here. Exact wont find the goal.
example : a * b = c * b → a = c := by exact mul_right_cancel
example : a * b = c * b ↔ a = c := by exact?
end Test_IsRightCancelMul
--E
--G: add_right_cancel
section Test_IsRightCancelAdd
variable [Add G] [IsRightCancelAdd G] {a b c : G}
--NOTE: Something is broken here. Exact wont find the goal.
example : a + b = c + b → a = c := by exact add_right_cancel
example : a + b = c + b ↔ a = c := by exact?
end Test_IsRightCancelAdd
--E
--T: Semigroup
example [Semigroup G] : ∀ a b c : G, a * b * c = a * (b * c) := Semigroup.mul_assoc
example [Semigroup G] : Mul G := inferInstance
--E
--T: AddSemigroup
example [AddSemigroup G] : ∀ a b c : G, a + b + c = a + (b + c) := AddSemigroup.add_assoc
example [AddSemigroup G] : Add G := inferInstance
--E
--T: CommMagma
example [CommMagma G] : ∀ a b : G, a * b = b * a := CommMagma.mul_comm
example [CommMagma G] : Mul G := inferInstance
--E
--T: AddCommMagma
example [AddCommMagma G] : ∀ a b : G, a + b = b + a := AddCommMagma.add_comm
example [AddCommMagma G] : Add G := inferInstance
--E
--T: CommSemigroup
example [CommSemigroup G] : Semigroup G := inferInstance
example [CommSemigroup G] : CommMagma G := inferInstance
--E
--T: AddCommSemigroup
example [AddCommSemigroup G] : AddSemigroup G := inferInstance
example [AddCommSemigroup G] : AddCommMagma G := inferInstance
--E
--G: commMagma_right_left_cancel
example [CommMagma G] [IsRightCancelMul G] : IsLeftCancelMul G := by exact?
example [AddCommMagma G] [IsRightCancelAdd G] : IsLeftCancelAdd G := by exact?
--E
--G: commMagma_left_right_cancel
example [CommMagma G] [IsLeftCancelMul G] : IsRightCancelMul G := by exact?
example [AddCommMagma G] [IsLeftCancelAdd G] : IsRightCancelAdd G := by exact?
--E
--G: commMagma_cancel
example [CommMagma G] [IsLeftCancelMul G] : IsCancelMul G := by exact?
example [AddCommMagma G] [IsLeftCancelAdd G] : IsCancelAdd G := by exact?
example [CommMagma G] [IsRightCancelMul G] : IsCancelMul G := by exact?
example [AddCommMagma G] [IsRightCancelAdd G] : IsCancelAdd G := by exact?
--E
--T: AddLeftCancelSemigroup
example [AddLeftCancelSemigroup G] : AddSemigroup G := inferInstance
example [AddLeftCancelSemigroup G] : ∀ a b c : G, a + b= a + c → b = c := AddLeftCancelSemigroup.add_left_cancel
--E
--NOTE: Skipping LeftCancelSemigroup satisfies IsLeftCancelMul

--T: RightCancelSemigroup
example [RightCancelSemigroup G] : Semigroup G := inferInstance
example [RightCancelSemigroup G] : ∀ a b c : G, a * b= c * b → a = c := RightCancelSemigroup.mul_right_cancel
--E
--T: AddRightCancelSemigroup
example [AddRightCancelSemigroup G] : AddSemigroup G := inferInstance
example [AddRightCancelSemigroup G] : ∀ a b c : G, a + b= c + b → a = c := AddRightCancelSemigroup.add_right_cancel
--E
--T: MulOneClass
example [MulOneClass G] : One G := inferInstance
example [MulOneClass G] : Mul G := inferInstance
example [MulOneClass G] : ∀ a : G, 1 * a = a := MulOneClass.one_mul
example [MulOneClass G] : ∀ a : G, a * 1 = a := MulOneClass.mul_one
--E
--T: AddZeroClass
example [AddZeroClass G] : Zero G := inferInstance
example [AddZeroClass G] : Add G := inferInstance
example [AddZeroClass G] : ∀ a : G, 0 + a = a := AddZeroClass.zero_add
example [AddZeroClass G] : ∀ a : G, a + 0 = a := AddZeroClass.add_zero
--E
--G: MulOneClass.ext
example {M : Type*} [MulOneClass M] {m₁ m₂ : MulOneClass M} : m₁.mul = m₂.mul → m₁ = m₂ := by exact?
--E
--G: AddZeroClass.ext
example {M : Type*} [AddZeroClass M] {m₁ m₂ : AddZeroClass M} : m₁.add = m₂.add → m₁ = m₂ := by exact?
--E
--T: npowRec
example [One M] [Mul M] : npowRec (0 : ℕ) (a : M) = 1 := rfl
example [One M] [Mul M] (n : ℕ) (a : M) : npowRec (n + 1) a = npowRec n a * a := rfl
--E
--T: nsmulRec
example [Zero M] [Add M] : nsmulRec (0 : ℕ) (a : M) = 0 := rfl
example [Zero M] [Add M] (n : ℕ) (a : M) : nsmulRec (n + 1) a = nsmulRec n a + a := rfl
--E
section Test_ns_funcs
variable {M : Type u}
--G: npowRec_add
section Test_npowRec
variable [One M] [Semigroup M] (m n : ℕ) (hn : n ≠ 0) (a : M) (ha : 1 * a = a)
include hn ha
example : npowRec (m + n) a = npowRec m a * npowRec n a := by exact?
example : npowRec (n + 1) a = a * npowRec n a := by exact?
end Test_npowRec
--E
--G: nsmulRec_add
section Test_nsmulRec
variable [Zero M] [AddSemigroup M] (m n : ℕ) (hn : n ≠ 0) (a : M) (ha : 0 + a = a)
include hn ha
example : nsmulRec (m + n) a = nsmulRec m a + nsmulRec n a := by exact?
example : nsmulRec (n + 1) a = a + nsmulRec n a := by exact?
end Test_nsmulRec
--E
end Test_ns_funcs
--NOTE: TODO: I am intentionally ignoring the complexity of npowRec' and forgetful inheritance.
--NOTE: There will a time when it is important. However, I want to walk the model through that process.

--T: Monoid
/-- A `Monoid` is a `Semigroup` with an element `1` such that `1 * a = a * 1 = a`. -/
example [Monoid G] : Semigroup G := inferInstance
example [Monoid G] : MulOneClass G := inferInstance
example [Monoid G] : ℕ → G → G := Monoid.npow
example [Monoid G] : ∀ x : G, Monoid.npow 0 x = 1 := Monoid.npow_zero
example [Monoid G] : ∀ (n : ℕ) (x : G), Monoid.npow (n + 1) x = Monoid.npow n x * x := Monoid.npow_succ
--E
--T: AddMonoid
/-- An `AddMonoid` is an `AddSemigroup` with an element `0` such that `0 + a = a + 0 = a`. -/
example [AddMonoid G] : AddSemigroup G := inferInstance
example [AddMonoid G] : AddZeroClass G := inferInstance
example [AddMonoid G] : ℕ → G → G := AddMonoid.nsmul
example [AddMonoid G] : ∀ x : G, AddMonoid.nsmul 0 x = 0 := AddMonoid.nsmul_zero
example [AddMonoid G] : ∀ (n : ℕ) (x : G), AddMonoid.nsmul (n + 1) x = AddMonoid.nsmul n x + x := AddMonoid.nsmul_succ
--E
--NOTE: OMITTED REST OF FILE. I AM BORED.

end Test_AlgebraGroupDefs
