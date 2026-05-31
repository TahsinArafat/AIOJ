CREATE TABLE IF NOT EXISTS remote_languages (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    platform VARCHAR(50) NOT NULL,
    local_id VARCHAR(50) NOT NULL,
    remote_id VARCHAR(50) NOT NULL,
    display_name VARCHAR(100) NOT NULL,
    enabled BOOLEAN DEFAULT true,
    sort_order INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(platform, local_id)
);

CREATE INDEX idx_remote_languages_platform ON remote_languages(platform);

INSERT INTO remote_languages (platform, local_id, remote_id, display_name, enabled, sort_order) VALUES
    ('codeforces', 'cpp-gpp-64', '54', 'GNU G++17 7.3.0 (64 bit)', true, 1),
    ('codeforces', 'cpp-gpp-32', '53', 'GNU G++14 6.4.0 (32 bit)', true, 2),
    ('codeforces', 'c-gcc-64', '43', 'GNU GCC C11 9.2.0', true, 3),
    ('codeforces', 'cpp-clang', '52', 'Clang++17', true, 4),
    ('codeforces', 'python', '70', 'Python 3.8.10', true, 5),
    ('codeforces', 'pypy', '41', 'PyPy 3.7.10', true, 6),
    ('codeforces', 'java', '60', 'Java 11.0.6', true, 7),
    ('codeforces', 'rust', '75', 'Rust 1.75.0', true, 8),
    ('codeforces', 'nodejs', '55', 'Node.js 18.16.1', true, 9),
    ('codeforces', 'csharp', '65', 'Mono C# 6.12.0', true, 10);
