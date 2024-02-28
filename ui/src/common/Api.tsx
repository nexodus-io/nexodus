export const backend = `${window.location.protocol}//api.${window.location.host}`;

export const fetchJson = (url: string, options: any = {}) => {
  options.credentials = "include";
  return fetch(url, options).then((response) => {
    if (!response.ok) {
      throw new Error(`Could not fetch ${url}, status: ${response.status}`);
    }
    if (response.status === 204) {
      return null;
    }
    return response.json();
  });
};
