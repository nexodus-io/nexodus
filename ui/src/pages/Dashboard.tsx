import {
    Button,
    Card,
    CardActions,
    CardContent,
    CardHeader,
    CardMedia,
} from '@mui/material';
import {FontAwesomeIcon} from "@fortawesome/react-fontawesome"
import {IconProp} from "@fortawesome/fontawesome-svg-core"
import {faApple, faWindows, faLinux} from "@fortawesome/free-brands-svg-icons"

import CardImage from "../apex.png"

const Dashboard = () => {

    return (
        <Card>
            <CardMedia
                component="img"
                height="200"
                image={CardImage}
                alt="apex mountain image"
            />
            <CardHeader title="Welcome to Apex" />
            <CardContent>
                Apex is a connectivity-as-a-service solution.
                To get started, please download the client.
            </CardContent>
            <CardActions>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faApple as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-darwin-amd64"
                >Download (x86_64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faApple as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-darwin-arm64"
                    >Download (aarch64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faWindows as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-windows-amd64"
                >Download (x86_64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faLinux as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-linux-arm64"
                >Download (x86_64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faLinux as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-linux-arm64"
                >Download (aarch64)</Button>
                <Button
                    size="small"
                    startIcon={<FontAwesomeIcon icon={faLinux as IconProp}/>}
                    href="https://apex-net.s3.amazonaws.com/apex-linux-arm64"
                >Download (arm)</Button>
            </CardActions>
        </Card>
    );
};

export default Dashboard;
