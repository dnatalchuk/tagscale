const React = require('react');
const { render, screen } = require('@testing-library/react');
const App = require('./App');

test('renders Dashboard Overview', () => {
  render(React.createElement(App));
  const text = screen.getByText(/Dashboard Overview/i);
  expect(text).toBeInTheDocument();
});
