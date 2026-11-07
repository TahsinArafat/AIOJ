-- Normalise problems.memory_limit back to kilobytes.
--
-- The column has always been KB: the admin create form and handler store the
-- value verbatim (default 262144 = 256 MB), the judge reads it as KB and
-- multiplies by 1024 itself, and the problem page renders memory_limit/1024 as
-- MB. Two writers disagreed and stored bytes instead:
--
--   * the legacy synchronous generator (internal/generate/service.go), whose
--     comment claimed "store expects bytes";
--   * ApplyService.Apply, which multiplied a KB proposal field by 1024.
--
-- The result was a problem page reading "262144000 MB" and a sandbox limit
-- 1024x larger than intended.
--
-- The write paths are fixed. This repairs rows they already wrote.
--
-- Safety: 4194304 KB (4 GiB) is the ceiling the harness enforces, so no
-- legitimate KB value exceeds it. Anything above is unambiguously bytes, and
-- the divisibility guard makes the division exact.
UPDATE problems
SET memory_limit = memory_limit / 1024
WHERE memory_limit > 4194304
  AND memory_limit % 1024 = 0;
