# Sub-Plan 20: Internationalization

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Support multiple languages in the UI with locale switching.

**Architecture:** Add i18n library, create translation files, update components to use translations.

**Tech Stack:** React, TypeScript, react-i18next

---

## File Structure

### Frontend Files to Create
- `web/src/i18n/index.ts` - i18n configuration
- `web/src/i18n/locales/en.json` - English translations
- `web/src/i18n/locales/bn.json` - Bengali translations
- `web/src/i18n/locales/ru.json` - Russian translations
- `web/src/components/LanguageSwitcher.tsx` - Language selector

### Frontend Files to Modify
- `web/src/App.tsx` - Add i18n provider
- `web/src/main.tsx` - Initialize i18n
- Multiple component files - Use translation keys

---

## Tasks

### Task 1: Install Dependencies

**Files:**
- Modify: `web/package.json`

- [ ] **Step 1: Install i18n packages**

Run:
```bash
cd web && npm install react-i18next i18next i18next-browser-languagedetector
```

- [ ] **Step 2: Commit**

```bash
git add web/package.json web/package-lock.json
git commit -m "feat(i18n): add i18n dependencies"
```

---

### Task 2: i18n Configuration

**Files:**
- Create: `web/src/i18n/index.ts`
- Create: `web/src/i18n/locales/en.json`
- Create: `web/src/i18n/locales/bn.json`
- Create: `web/src/i18n/locales/ru.json`

- [ ] **Step 1: Create i18n configuration**

```typescript
// web/src/i18n/index.ts
import i18n from 'i18next';
import { initReactI18next } from 'react-i18next';
import LanguageDetector from 'i18next-browser-languagedetector';

import en from './locales/en.json';
import bn from './locales/bn.json';
import ru from './locales/ru.json';

i18n
  .use(LanguageDetector)
  .use(initReactI18next)
  .init({
    resources: {
      en: { translation: en },
      bn: { translation: bn },
      ru: { translation: ru },
    },
    fallbackLng: 'en',
    interpolation: {
      escapeValue: false,
    },
  });

export default i18n;
```

- [ ] **Step 2: Create English translations**

```json
// web/src/i18n/locales/en.json
{
  "common": {
    "loading": "Loading...",
    "error": "Error",
    "save": "Save",
    "cancel": "Cancel",
    "delete": "Delete",
    "edit": "Edit",
    "create": "Create",
    "search": "Search...",
    "noResults": "No results found",
    "submit": "Submit",
    "back": "Back",
    "next": "Next",
    "prev": "Previous"
  },
  "nav": {
    "home": "Home",
    "problems": "Problems",
    "contests": "Contests",
    "gym": "Gym",
    "groups": "Groups",
    "teams": "Teams",
    "blog": "Blog",
    "profile": "Profile",
    "submissions": "My Submissions",
    "login": "Login",
    "register": "Register",
    "logout": "Logout"
  },
  "home": {
    "title": "AIOJ",
    "subtitle": "Lightweight Online Judge for Competitive Programming",
    "browseProblems": "Browse Problems",
    "viewContests": "View Contests"
  },
  "problems": {
    "title": "Problems",
    "difficulty": "Difficulty",
    "tags": "Tags",
    "timeLimit": "Time Limit",
    "memoryLimit": "Memory Limit",
    "solved": "Solved",
    "submissions": "Submissions",
    "acceptance": "Acceptance",
    "submit": "Submit",
    "verdict": "Verdict",
    "language": "Language"
  },
  "contests": {
    "title": "Contests",
    "startTime": "Start Time",
    "endTime": "End Time",
    "duration": "Duration",
    "register": "Register",
    "registered": "Registered",
    "scoreboard": "Scoreboard",
    "problems": "Problems",
    "running": "Running",
    "upcoming": "Upcoming",
    "ended": "Ended"
  },
  "auth": {
    "login": "Login",
    "register": "Register",
    "username": "Username",
    "email": "Email",
    "password": "Password",
    "confirmPassword": "Confirm Password",
    "forgotPassword": "Forgot Password?",
    "noAccount": "Don't have an account?",
    "hasAccount": "Already have an account?"
  },
  "profile": {
    "title": "Profile",
    "rating": "Rating",
    "problemsSolved": "Problems Solved",
    "submissions": "Submissions",
    "bio": "Bio",
    "editProfile": "Edit Profile"
  },
  "errors": {
    "notFound": "Page not found",
    "unauthorized": "Please login first",
    "forbidden": "You don't have permission",
    "serverError": "Server error, please try again"
  }
}
```

- [ ] **Step 3: Create Bengali translations**

```json
// web/src/i18n/locales/bn.json
{
  "common": {
    "loading": "লোড হচ্ছে...",
    "error": "ত্রুটি",
    "save": "সংরক্ষণ",
    "cancel": "বাতিল",
    "delete": "মুছুন",
    "edit": "সম্পাদনা",
    "create": "তৈরি করুন",
    "search": "অনুসন্ধান...",
    "noResults": "কোনো ফলাফল পাওয়া যায়নি",
    "submit": "জমা দিন",
    "back": "পিছনে",
    "next": "পরবর্তী",
    "prev": "পূর্ববর্তী"
  },
  "nav": {
    "home": "হোম",
    "problems": "সমস্যা",
    "contests": "প্রতিযোগিতা",
    "gym": "জিম",
    "groups": "গ্রুপ",
    "teams": "দল",
    "blog": "ব্লগ",
    "profile": "প্রোফাইল",
    "submissions": "আমার সাবমিশন",
    "login": "লগইন",
    "register": "নিবন্ধন",
    "logout": "লগআউট"
  },
  "home": {
    "title": "AIOJ",
    "subtitle": "প্রতিযোগিতামূলক প্রোগ্রামিংয়ের জন্য হালকা অনলাইন জজ",
    "browseProblems": "সমস্যা ব্রাউজ করুন",
    "viewContests": "প্রতিযোগিতা দেখুন"
  },
  "problems": {
    "title": "সমস্যা",
    "difficulty": "কঠিনতা",
    "tags": "ট্যাগ",
    "timeLimit": "সময় সীমা",
    "memoryLimit": "মেমরি সীমা",
    "solved": "সমাধান হয়েছে",
    "submissions": "সাবমিশন",
    "acceptance": "গৃহীত",
    "submit": "জমা দিন",
    "verdict": "রায়",
    "language": "ভাষা"
  }
}
```

- [ ] **Step 4: Create Russian translations**

```json
// web/src/i18n/locales/ru.json
{
  "common": {
    "loading": "Загрузка...",
    "error": "Ошибка",
    "save": "Сохранить",
    "cancel": "Отмена",
    "delete": "Удалить",
    "edit": "Редактировать",
    "create": "Создать",
    "search": "Поиск...",
    "noResults": "Ничего не найдено",
    "submit": "Отправить",
    "back": "Назад",
    "next": "Далее",
    "prev": "Назад"
  },
  "nav": {
    "home": "Главная",
    "problems": "Задачи",
    "contests": "Контесты",
    "gym": "Гимназия",
    "groups": "Группы",
    "teams": "Команды",
    "blog": "Блог",
    "profile": "Профиль",
    "submissions": "Мои посылки",
    "login": "Войти",
    "register": "Регистрация",
    "logout": "Выйти"
  }
}
```

- [ ] **Step 5: Commit**

```bash
git add web/src/i18n/
git commit -m "feat(i18n): add i18n configuration and translations"
```

---

### Task 3: Language Switcher Component

**Files:**
- Create: `web/src/components/LanguageSwitcher.tsx`

- [ ] **Step 1: Create language switcher**

```tsx
// web/src/components/LanguageSwitcher.tsx
import { useTranslation } from 'react-i18next';

const languages = [
  { code: 'en', name: 'English', flag: '🇺🇸' },
  { code: 'bn', name: 'বাংলা', flag: '🇧🇩' },
  { code: 'ru', name: 'Русский', flag: '🇷🇺' },
];

export default function LanguageSwitcher() {
  const { i18n } = useTranslation();

  return (
    <select
      value={i18n.language}
      onChange={(e) => i18n.changeLanguage(e.target.value)}
      className="border rounded px-2 py-1 text-sm bg-white"
    >
      {languages.map((lang) => (
        <option key={lang.code} value={lang.code}>
          {lang.flag} {lang.name}
        </option>
      ))}
    </select>
  );
}
```

- [ ] **Step 2: Commit**

```bash
git add web/src/components/LanguageSwitcher.tsx
git commit -m "feat(i18n): add language switcher component"
```

---

### Task 4: Initialize i18n

**Files:**
- Modify: `web/src/main.tsx`
- Modify: `web/src/App.tsx`

- [ ] **Step 1: Import i18n in main.tsx**

```tsx
// web/src/main.tsx
import './i18n'; // Add this import
import React from 'react';
import ReactDOM from 'react-dom/client';
import App from './App';
import './global.css';

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>
);
```

- [ ] **Step 2: Add LanguageSwitcher to Navbar**

```tsx
// web/src/App.tsx
import LanguageSwitcher from './components/LanguageSwitcher';
import { useTranslation } from 'react-i18next';

function Navbar() {
  const { t } = useTranslation();
  // ... existing code ...
  
  return (
    <nav className="...">
      <div className="flex gap-6 items-center">
        <Link to="/" className="font-bold text-blue-600 text-lg">AIOJ</Link>
        <Link to="/problems" className="text-sm text-gray-600 hover:text-black">
          {t('nav.problems')}
        </Link>
        <Link to="/contests" className="text-sm text-gray-600 hover:text-black">
          {t('nav.contests')}
        </Link>
        {/* ... other links ... */}
      </div>
      <div className="flex gap-3 items-center">
        <LanguageSwitcher />
        {/* ... existing auth buttons ... */}
      </div>
    </nav>
  );
}
```

- [ ] **Step 3: Update Home page with translations**

```tsx
function Home() {
  const { t } = useTranslation();
  
  return (
    <div className="text-center py-24">
      <h1 className="text-4xl font-bold mb-3">{t('home.title')}</h1>
      <p className="text-gray-500">{t('home.subtitle')}</p>
      <div className="mt-8 flex justify-center gap-4">
        <Link to="/problems" className="bg-blue-600 text-white px-6 py-2.5 rounded hover:bg-blue-700">
          {t('home.browseProblems')}
        </Link>
        <Link to="/contests" className="border border-gray-300 px-6 py-2.5 rounded hover:bg-gray-50">
          {t('home.viewContests')}
        </Link>
      </div>
    </div>
  );
}
```

- [ ] **Step 4: Commit**

```bash
git add web/src/main.tsx web/src/App.tsx
git commit -m "feat(i18n): initialize i18n and update components"
```

---

## Verification Checklist

- [ ] Language switcher appears in navbar
- [ ] Switching language updates UI
- [ ] Language persists after refresh
- [ ] All major sections translated
- [ ] Fallback to English works

---

## Notes

1. **Languages**: English (en), Bengali (bn), Russian (ru)
2. **Detection**: Auto-detect from browser
3. **Storage**: Language preference in localStorage
4. **Fallback**: English as default
