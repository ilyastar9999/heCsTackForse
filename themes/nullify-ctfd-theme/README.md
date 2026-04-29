<p align="center">
  <img src="https://nullify.uno/assets/images/nullify_banner.svg">
</p>

# Nullify CTFd Theme

A theme for CTFd with the Nullify color scheme. (Mainly dark green and dark grey. We are bug dark theme supporters here)

## Installation

Navigate to your CTFd themes folder (/var/www/CTFd/CTFd/themes if you're following the HSCTF Platform Setup Guide) and execute this one-liner in that directory:

```
git clone https://github.com/AaronVigal/nullify-ctfd-theme.git && mv ./nullify-ctfd-theme/nullify/ ../ && rm -rf nullify-ctfd-theme/
```

Next, restart the ctfd service by executing `sudo systemctl restart ctfd` to load the new theme in.

Now, go into the CTFd Admin Panel > Config > Theme and select "nullify" for the theme. You'll also want to upload some cool logos to go on the page. Here are the ones we used. You should be able to steal them from the Nullify website if you don't have them. They're in like `/assets/images/` or something.

![CTFd Theme Page](https://i.imgur.com/pFhRGGn.png)

Now if you navigate back to the front end, you should have a cool dark-themed, green Nullify-looking platform.

![Cool Challenges Page](https://i.imgur.com/pWJMwPx.png)
