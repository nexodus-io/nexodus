import Dashboard from "../src/pages/Dashboard.js";

export default {
    url: process.env.NEXODUS_URL || 'https://try.nexodus.127.0.0.1.nip.io',
    username: process.env.NEXODUS_USERNAME || 'admin',
    password: process.env.NEXODUS_PASSWORD || 'floofykittens',
};
