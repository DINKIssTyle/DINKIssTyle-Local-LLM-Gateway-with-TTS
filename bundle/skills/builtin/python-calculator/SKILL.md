---
name: python-calculator
description: Calculate complex mathematics, date/time arithmetic, D-days, calendar lookups, ages, statistics, interest/loan rates, and precise numerical evaluations using Python. Use for requests involving 날짜 계산, D-day, 며칠 남았는지, 며칠 전/후, 요일 계산, 만 나이 계산, 복잡한 수학 연산, 백분율, 복리/이자 계산, 통계, or when exact numerical precision is needed.
---

# Python Calculator & Date/Math Skill

Use Python via the `execute_command` tool to perform exact date/time arithmetic, statistical calculations, complex mathematical formulas, and business/financial computations without relying on mental estimation.

## When to Use

- **Date & Time Calculations**:
  - D-Day calculations, days between two dates (예: "오늘부터 100일 뒤는?", "2024년 5월 5일까지 며칠 남았어?").
  - Weekday determinations (예: "1988년 9월 17일은 무슨 요일이었어?").
  - Leap year, age, or month/year arithmetic (예: "만 나이 계산", "근속 개월 수 계산").
- **Complex Mathematics & Numerical Precision**:
  - Multi-step algebraic expressions, trigonometry, combinatorics, large exponentiation/factorials (`math`, `decimal`).
  - Compound interest, loan repayment schedules, amortization (`decimal`).
  - Statistical summaries, averages, standard deviations, distributions.

## Workflow

1. **Formulate a Python One-Liner Script**:
   Write a self-contained Python script using standard libraries (`datetime`, `math`, `calendar`, `decimal`, `statistics`).
   Avoid external third-party dependencies (`numpy`, `pandas`) unless confirmed available.

2. **Execute via `execute_command`**:
   Run the command using `python3 -c "..."` (or `python -c "..."` depending on the environment).

   ### Common Execution Templates

   - **Date Arithmetic & D-Day**:
     ```bash
     python3 -c "from datetime import date, timedelta; today = date.today(); target = date(2026, 12, 31); diff = (target - today).days; print(f'D-day: {diff}일')"
     ```

   - **Future / Past Date Calculation**:
     ```bash
     python3 -c "from datetime import date, timedelta; print('100일 뒤:', date.today() + timedelta(days=100))"
     ```

   - **Day of the Week (Korean Weekdays)**:
     ```bash
     python3 -c "import datetime; weekdays = ['월요일', '화요일', '수요일', '목요일', '금요일', '토요일', '일요일']; d = datetime.date(1988, 9, 17); print(f'{d}는 {weekdays[d.weekday()]}입니다.')"
     ```

   - **Age Calculation (만 나이)**:
     ```bash
     python3 -c "from datetime import date; b = date(1990, 5, 20); t = date.today(); age = t.year - b.year - ((t.month, t.day) < (b.month, b.day)); print(f'만 {age}세')"
     ```

   - **Compound Interest / Financial Formula**:
     ```bash
     python3 -c "p = 10000000; r = 0.035; n = 12; t = 3; total = p * ((1 + r/n)**(n*t)); print(f'원리합계: {total:,.0f}원')"
     ```

   - **Exact Mathematics**:
     ```bash
     python3 -c "import math; print('결과:', math.factorial(20) / (math.factorial(5) * math.factorial(15)))"
     ```

3. **Verify and Format the Response**:
   - Inspect the standard output returned by the command.
   - Present the final calculated result clearly and concisely in the user's requested language (Korean).
   - If relevant, show the key formula or date breakdown so the user can easily verify.

## Accuracy Rules

- Never guess or approximate date differences or large arithmetic in text; always verify via Python execution.
- Use `CURRENT_TIME` provided in the environment as the baseline reference date if `date.today()` is referenced.
- Keep the Python code clean, safe, and free of destructive system calls.
