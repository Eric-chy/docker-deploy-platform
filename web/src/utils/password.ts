export function validatePassword(password: string): boolean {
  if (password.length < 8 || password.length > 18) return false;

  let digit = false;
  let lowercase = false;
  let uppercase = false;
  let symbol = false;
  for (const char of password) {
    if (char >= "0" && char <= "9") digit = true;
    else if (char >= "a" && char <= "z") lowercase = true;
    else if (char >= "A" && char <= "Z") uppercase = true;
    else if (
      (char >= "!" && char <= "/") ||
      (char >= ":" && char <= "@") ||
      (char >= "[" && char <= "`") ||
      (char >= "{" && char <= "~")
    ) {
      symbol = true;
    } else {
      return false;
    }
  }

  return [digit, lowercase, uppercase, symbol].filter(Boolean).length >= 2;
}
