-- Requires: ["Simple.lean"]
import Corelib

universe u w 

namespace Rat

-- S: Foo
example (n₁ d₁ n₂ d₂ : ℤ) : (n₁ /. d₁) * (n₂ /. d₂) = (n₁ * n₂) /. (d₁ * d₂) := by exact?
-- E

end Rat
