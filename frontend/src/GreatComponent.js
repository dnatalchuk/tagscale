const React = require('react');

function GreatComponent() {
  const [data, setData] = React.useState(null);

  React.useEffect(() => {
    const headers = {};
    const apiKey = process.env.REACT_APP_API_KEY;
    if (apiKey) {
      headers['Authorization'] = `Bearer ${apiKey}`;
    }

    fetch('/api/v1/dashboard/overview', { headers })
      .then(res => res.json())
      .then(setData)
      .catch(err => console.error('Failed to fetch overview', err));
  }, []);

  if (!data) {
    return React.createElement('div', null, 'Loading...');
  }

  return React.createElement('pre', null, JSON.stringify(data, null, 2));
}

module.exports = GreatComponent;
