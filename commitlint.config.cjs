module.exports = {
  extends: ['@commitlint/config-conventional'],
  rules: {
    'scope-enum': [
      1, // warning
      'always',
      ['backend', 'cli', 'android', 'ci', 'docs', 'deps', 'installer', 'release'],
    ],
  },
};
